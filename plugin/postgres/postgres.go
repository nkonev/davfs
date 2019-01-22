package postgres

import (
	"database/sql"
	"encoding/hex"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/nkonev/davfs"
	"golang.org/x/net/context"
	"golang.org/x/net/webdav"
)

const createSQL = `
create table if not exists filesystem(
	name text not null,
	content text not null,
	mode bigint not null,
	mod_time timestamp not null,
	primary key (name)
);

insert into filesystem(name, content, mode, mod_time) values('/', '', 2147484159, current_timestamp) on conflict DO NOTHING;
`

func init() {
	davfs.Register("postgres", &Driver{})
}

type Driver struct {
}

type PostgresFileSystem struct {
	db *sql.DB
	mu sync.Mutex
}

type FileInfo struct {
	name     string
	size     int64
	mode     os.FileMode
	mod_time time.Time
}

type File struct {
	fs       *PostgresFileSystem
	name     string
	off      int64
	children []os.FileInfo
}

func (d *Driver) Mount(source string) (webdav.FileSystem, error) {
	db, err := sql.Open("postgres", source)
	if err != nil {
		return nil, err
	}
	return &PostgresFileSystem{db: db}, nil
}

func (d *Driver) CreateFS(source string) error {
	db, err := sql.Open("postgres", source)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(createSQL)
	if err != nil {
		return err
	}
	return nil
}

func clearName(name string) (string, error) {
	slashed := strings.HasSuffix(name, "/")
	name = path.Clean(name)
	if !strings.HasSuffix(name, "/") && slashed {
		name += "/"
	}
	if !strings.HasPrefix(name, "/") {
		return "", os.ErrInvalid
	}
	return name, nil
}

func (fs *PostgresFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log.Debugf("PostgresFileSystem.Mkdir %v\n", name)

	if !strings.HasSuffix(name, "/") {
		name += "/"
	}

	var err error
	if name, err = clearName(name); err != nil {
		log.Printf("Unexpected error during mkdir: %v\n", err)
		return err
	}

	_, err = fs.stat(name)
	if err == nil {
		return os.ErrExist
	}
	if err != nil && err != os.ErrNotExist {
		log.Printf("Unexpected error during mkdir: %v\n", err)
		return err
	}

	base := "/"
	for _, elem := range strings.Split(strings.Trim(name, "/"), "/") {
		base += elem + "/"
		_, err = fs.stat(base)
		if err != os.ErrNotExist {
			log.Printf("Unexpected error during mkdir: %v\n", err)
			return err
		}
		_, err = fs.db.Exec(`insert into filesystem(name, content, mode, mod_time) values($1, '', $2, current_timestamp)`, base, perm.Perm()|os.ModeDir)
		if err != nil {
			log.Printf("Unexpected error during mkdir: %v\n", err)
			return err
		}
	}
	return nil
}

func (fs *PostgresFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log.Debugf("PostgresFileSystem.OpenFile %v\n", name)

	var err error
	if name, err = clearName(name); err != nil {
		return nil, err
	}

	if flag&os.O_CREATE != 0 {
		// file should not have / suffix.
		if strings.HasSuffix(name, "/") {
			return nil, os.ErrInvalid
		}
		// based directory should be exists.
		dir, _ := path.Split(name)
		_, err := fs.stat(dir)
		if err != nil {
			return nil, os.ErrInvalid
		}
		_, err = fs.stat(name)
		if err == nil {
			if flag&os.O_EXCL != 0 {
				return nil, os.ErrExist
			}
			fs.removeAll(name)
		}
		_, err = fs.db.Exec(`insert into filesystem(name, content, mode, mod_time) values($1, '', $2, current_timestamp)`, name, perm.Perm())
		if err != nil {
			return nil, err
		}
		return &File{fs, name, 0, nil}, nil
	}

	fi, err := fs.stat(name)
	if err != nil {
		return nil, os.ErrNotExist
	}
	if !strings.HasSuffix(name, "/") && fi.IsDir() {
		name += "/"
	}
	return &File{fs, name, 0, nil}, nil
}

func (fs *PostgresFileSystem) removeAll(name string) error {
	var err error
	if name, err = clearName(name); err != nil {
		return err
	}

	fi, err := fs.stat(name)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		_, err = fs.db.Exec(`delete from filesystem where name like $1 escape '\'`, strings.Replace(name, `%`, `\%`, -1)+`%`)
	} else {
		_, err = fs.db.Exec(`delete from filesystem where name = $1`, name)
	}
	return err
}

func (fs *PostgresFileSystem) RemoveAll(ctx context.Context, name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log.Debugf("PostgresFileSystem.RemoveAll %v\n", name)

	return fs.removeAll(name)
}

func (fs *PostgresFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log.Debugf("PostgresFileSystem.Rename %v %v\n", oldName, newName)

	var err error
	if oldName, err = clearName(oldName); err != nil {
		return err
	}
	if newName, err = clearName(newName); err != nil {
		return err
	}

	of, err := fs.stat(oldName)
	if err != nil {
		return os.ErrExist
	}
	if of.IsDir() && !strings.HasSuffix(oldName, "/") {
		oldName += "/"
		newName += "/"
	}

	_, err = fs.stat(newName)
	if err == nil {
		return os.ErrExist
	}

	_, err = fs.db.Exec(`update filesystem set name = $1 where name = $2`, newName, oldName)
	return err
}

func (fs *PostgresFileSystem) stat(name string) (os.FileInfo, error) {
	var err error
	if name, err = clearName(name); err != nil {
		return nil, err
	}

	rows, err := fs.db.Query(`select name, length(content)/2, mode, mod_time from filesystem where name = $1`, name)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		if strings.HasSuffix(name, "/") {
			return nil, os.ErrNotExist
		}
		rows, err = fs.db.Query(`select name, length(content)/2, mode, mod_time from filesystem where name = $1`, name+"/")
		if err != nil {
			return nil, err
		}
		if !rows.Next() {
			rows.Close()
			return nil, os.ErrNotExist
		}
	}
	defer rows.Close()
	var fi FileInfo
	err = rows.Scan(&fi.name, &fi.size, &fi.mode, &fi.mod_time)
	if err != nil {
		return nil, err
	}
	_, fi.name = path.Split(path.Clean(fi.name))
	if fi.name == "" {
		fi.name = "/"
		fi.mod_time = time.Now()
	}
	return &fi, nil
}

func (fs *PostgresFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log.Debugf("PostgresFileSystem.Stat %v\n", name)

	return fs.stat(name)
}

func (fi *FileInfo) Name() string       { return fi.name }
func (fi *FileInfo) Size() int64        { return fi.size }
func (fi *FileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *FileInfo) ModTime() time.Time { return fi.mod_time }
func (fi *FileInfo) IsDir() bool        { return fi.mode.IsDir() }
func (fi *FileInfo) Sys() interface{}   { return nil }

func (f *File) Write(p []byte) (int, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	log.Debugf("File.Write %v\n", f.name)

	_, err := f.fs.db.Exec(`update filesystem set content = substr(content, 1, $1) || $2 where name = $3`, f.off*2, hex.EncodeToString(p), f.name)
	if err != nil {
		return 0, err
	}
	f.off += int64(len(p))
	return len(p), err
}

func (f *File) Close() error {
	log.Debugf("File.Close %v\n", f.name)

	return nil
}

func (f *File) Read(p []byte) (int, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	log.Debugf("File.Read %v\n", f.name)

	rows, err := f.fs.db.Query(`select mode, substr(content, $1, $2) from filesystem where name = $3`, 1+f.off*2, len(p)*2, f.name)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if !rows.Next() {
		return 0, os.ErrInvalid
	}
	var content string
	var mode os.FileMode
	err = rows.Scan(&mode, &content)
	if err != nil {
		return 0, err
	}
	if mode.IsDir() {
		return 0, os.ErrInvalid
	}
	b, err := hex.DecodeString(content)
	if err != nil {
		return 0, err
	}
	copy(p, b)
	bl := len(b)
	f.off += int64(bl)
	if bl == 0 {
		return 0, io.EOF
	}
	return bl, nil
}

func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	log.Debugf("File.Readdir %v\n", f.name)

	if f.children == nil {
		rows, err := f.fs.db.Query(`select name from filesystem where name <> $1 and name like $2 escape '\'`, f.name, strings.Replace(f.name, `%`, `\%`, -1)+"%")
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		f.children = []os.FileInfo{}
		for rows.Next() {
			var name string
			err = rows.Scan(&name)
			if err != nil {
				return nil, err
			}
			part := strings.TrimRight(name[len(f.name):], "/")
			if strings.IndexRune(part, '/') != -1 {
				continue
			}
			fi, err := f.fs.stat(name)
			if err != nil {
				return nil, err
			}
			f.children = append(f.children, fi)
		}
	}

	old := f.off
	if old >= int64(len(f.children)) {
		if count > 0 {
			return nil, io.EOF
		}
		return nil, nil
	}
	if count > 0 {
		f.off += int64(count)
		if f.off > int64(len(f.children)) {
			f.off = int64(len(f.children))
		}
	} else {
		f.off = int64(len(f.children))
		old = 0
	}
	return f.children[old:f.off], nil
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	log.Debugf("File.Seek %v %v %v\n", f.name, offset, whence)

	var err error
	switch whence {
	case 0:
		f.off = 0
	case 2:
		if fi, err := f.fs.stat(f.name); err != nil {
			return 0, err
		} else {
			f.off = fi.Size()
		}
	}
	f.off += offset
	return f.off, err
}

func (f *File) Stat() (os.FileInfo, error) {
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()

	log.Debugf("File.Stat %v\n", f.name)

	return f.fs.stat(f.name)
}
