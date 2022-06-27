// Code generated by go-bindata. (@generated) DO NOT EDIT.

 //Package migrations generated by go-bindata.// sources:
// migrations/001_init.down.sql
// migrations/001_init.up.sql
// migrations/migrations_test.go
package migrations

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// ModTime return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var __001_initDownSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x72\x09\xf2\x0f\x50\x08\x71\x74\xf2\x71\x55\x28\xae\x2c\x2e\x49\xcd\x8d\x2f\x49\x4c\xca\x49\x2d\xb6\x06\x04\x00\x00\xff\xff\x63\xc3\x98\x16\x19\x00\x00\x00")

func _001_initDownSqlBytes() ([]byte, error) {
	return bindataRead(
		__001_initDownSql,
		"001_init.down.sql",
	)
}

func _001_initDownSql() (*asset, error) {
	bytes, err := _001_initDownSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "001_init.down.sql", size: 25, mode: os.FileMode(436), modTime: time.Unix(1639565921, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var __001_initUpSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xbc\x94\x41\x6f\x9c\x30\x10\x85\xef\xfc\x8a\xe9\x29\xac\xc4\x61\xef\x55\x2a\x51\x6a\x52\x14\x4a\x2a\xe2\x48\xc9\xc9\x62\xed\xe9\xae\x55\xd6\x46\x63\x53\x91\x7f\x5f\xed\x2e\x51\x68\x0a\x2c\x5b\xa9\xe1\x88\xdf\x78\xe6\x7d\x6f\xe4\xa4\x64\x31\x67\xc0\xe3\xcf\x39\x83\x2c\x85\xe2\x8e\x03\x7b\xcc\xee\xf9\x3d\x10\x6e\xb5\xf3\xf4\x0c\x61\x00\x00\xa0\x15\x64\x05\x67\x37\xac\x3c\x8a\x8a\x87\x3c\x8f\x8e\x07\xce\x53\x2b\x7d\x4b\x08\x9c\x3d\xf2\x37\x87\xd2\x1a\x4f\xb6\xae\x91\xc6\x4e\x1b\xc2\x1f\xba\x1b\xad\x23\xac\x3c\x2a\x51\xf9\x89\xae\x72\x57\x69\x23\x5e\x87\x8a\x82\xe3\xef\xef\x65\xf6\x2d\x2e\x9f\xe0\x96\x3d\x85\x2f\x92\x08\xb4\x5a\x9d\xaa\x92\xaf\x2c\xb9\x0d\xb5\x82\x4f\xd7\xb0\x5e\x05\xab\x8f\x41\x30\x43\xc0\x3d\x3b\x8f\x7b\x51\xc9\xba\x67\xe0\xab\x4d\x8d\x62\x92\xc4\x39\xb3\xfa\x97\xae\x71\x8b\xee\x50\x7e\xc6\xce\xa5\x38\xda\x46\xbd\x11\xcc\x02\x79\x31\x12\x0d\x66\xee\x11\xa5\x77\x25\xcb\x6e\x8a\x89\x82\x15\x94\x2c\x65\x25\x2b\x12\xf6\xba\x21\x7f\x92\x5e\x48\x75\x00\x2b\x5c\x82\xe0\x9f\xd9\x2f\xc0\x30\xdc\x0e\x08\x07\x57\x7d\xb8\x86\xab\x75\xb7\x5e\xf8\x5d\xbd\x33\x42\xdf\x19\x41\x28\x51\x37\xde\x2d\x83\xb8\xa9\xad\xfc\x29\x4c\xbb\xdf\x20\xcd\x4a\x2c\xa9\x49\xc5\xa1\xed\xae\x72\xbb\xb1\x25\x47\x22\x7b\x4a\x60\x3c\xb5\xf9\x38\xfa\x8b\x2f\xf1\xdf\x90\x95\xe8\x9c\x9d\xdc\xa2\x41\xab\x4b\x60\x2c\x1c\xa1\x41\xa3\xb4\xd9\x0a\xdf\x2d\x0b\xa0\x52\x8a\xd0\xb9\x31\x74\x53\x48\x8d\x35\x12\xa7\xc2\x6a\xf7\x8d\x68\x48\x4b\x14\xd2\xb6\xe6\xef\xd7\x01\xbe\xb0\x34\x7e\xc8\x39\xac\xff\xcf\x43\xd2\xfb\x89\x4e\x53\x1e\x82\xfb\x1d\x00\x00\xff\xff\x8b\xf1\x0b\x5d\x52\x06\x00\x00")

func _001_initUpSqlBytes() ([]byte, error) {
	return bindataRead(
		__001_initUpSql,
		"001_init.up.sql",
	)
}

func _001_initUpSql() (*asset, error) {
	bytes, err := _001_initUpSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "001_init.up.sql", size: 1618, mode: os.FileMode(436), modTime: time.Unix(1655499388, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _migrations_testGo = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xac\x54\x5d\x6f\xdb\x46\x10\x7c\xbe\xfb\x15\x0b\xa2\xa8\x49\x47\x26\x25\xc5\x41\x5a\x03\x7a\xb0\x14\x05\x0d\x60\x2b\xad\xed\xc2\x0f\xfd\x50\xce\xe4\x8a\xbc\x8a\xbc\xa3\xef\x96\xa6\x84\x22\xfd\xed\xc5\x1d\x69\xd9\x72\x5b\x21\x45\xfb\x24\x60\x6f\x67\x67\x77\x66\xa8\x5a\xa4\x6b\x91\x23\x54\x32\x37\x82\xa4\x56\x76\x49\x68\x89\x73\x59\xd5\xda\x10\x84\x9c\x05\xa9\x56\x84\x1b\x0a\x38\x0b\xdc\x9b\x54\x79\xc0\x39\x0b\x72\x49\x45\x73\x17\xa7\xba\x4a\x7e\x13\xe9\x3a\x4d\xea\x7c\x93\x3c\x9c\xba\x9f\x5a\xeb\x32\xd8\x6f\xb1\x64\x90\xd2\xc2\x24\x7e\xc4\x6a\x9b\x18\xbc\x6f\xa4\xc1\x17\x6d\x8e\x48\x96\x28\x75\x92\xeb\x13\x12\x77\x25\x96\x42\x65\x89\x54\x84\x46\x89\x32\xd9\x95\xbe\x0c\x57\xaf\xf3\xc4\xde\x97\x96\xb4\xc1\x44\x56\x75\x99\xd8\xad\x25\xac\xbe\x0c\xed\x56\xb5\x01\x8f\x38\x5f\x35\x2a\x85\x1b\xb4\x74\xd9\x94\x24\xd3\x42\x48\x75\xf9\xa8\x58\x48\x70\xdc\xeb\x12\xdf\x44\xf0\x3b\x67\x8d\x29\xe1\x6c\x02\x1e\x1e\x7f\xaf\x2d\xe5\x06\xed\x8f\x57\x17\xb7\x92\x8a\x0f\x95\xc8\x31\xa4\x01\x04\x3d\xe9\xd3\x49\x27\x75\xdf\x1a\x0c\x20\x18\x0f\xc7\xe3\xe1\x9b\xe1\xe9\x72\x34\x1a\x8e\x4f\xdf\xba\xd2\xd3\xed\x11\x67\x29\x6d\x1c\x47\xef\x4d\x3c\x15\xe9\x3a\x37\xba\x51\x59\x18\x71\xe6\xf4\x1f\x00\x1a\xe3\x5a\x7a\x3f\xe2\x99\x56\x0a\x53\x0a\x53\xda\x0c\xa0\x31\x65\xc4\x59\xef\x41\xbc\xd0\x73\x63\xb4\x71\x6b\xa1\x31\x11\x67\x19\xae\xd0\x40\x07\x2b\xb5\xc5\x30\xe2\x9c\x3d\x08\x03\xb8\xc1\xf4\x87\x06\xcd\x76\xa6\x1b\x45\x30\x01\xa7\x4c\x78\xef\x2a\x60\xc9\x48\x95\x47\x20\x15\x39\x15\x98\xd1\xad\xe7\x77\x53\x3c\xe6\x4a\xb7\x1d\xbb\xef\x8f\x38\xf3\x23\x75\x99\x2d\x44\x85\xd6\xe1\x38\x63\x6e\xeb\x09\x18\xdd\xc6\xd7\xa9\x50\xe1\xd7\x8f\xcf\xae\xfd\x1f\xf7\x65\x06\xa9\x31\x6a\x37\x8b\xb3\xcf\x9c\xb3\x24\x81\x51\x0c\xb3\x02\xd3\x35\x50\x21\x08\x5a\x84\x42\x3c\x20\x78\x25\x2d\xb4\x92\x0a\xa0\x02\x1d\x0c\x42\xa5\xd5\x89\x77\xf6\xc3\x3b\xb0\xa9\xae\x31\x8b\x60\xa5\x4d\x25\x28\xe6\xcc\x6f\xfc\xb1\xcc\xde\xfb\x02\x9c\x4d\x38\x63\x9f\xae\xe7\x17\xf3\xd9\x0d\xa4\x4e\x8a\xf0\x38\x82\xf7\x57\x1f\x2f\x41\xaa\x0e\x24\xb5\x5a\xda\xb4\xc0\x4a\xc4\x3d\xdd\xed\x77\xf3\xab\x79\xc7\xbd\x54\xa2\x42\xf8\x03\x8e\x7e\x5d\xfe\x34\x3c\xf9\xf6\x97\x57\x5f\x1d\xc1\xf9\xe2\x5d\xff\x48\xdb\x1a\x27\x47\xd3\xf3\xeb\x39\xdc\x9c\x4f\x2f\xe6\x47\x9f\x38\x7b\xbc\xcc\xeb\x3e\xc5\x95\x36\xb8\x4b\xa0\x53\x79\xdf\x98\x70\x7f\xdf\x67\x56\xcf\xef\x1b\x51\x3a\xe1\xde\x8c\xbe\x19\xc0\xa1\xa9\x51\xa7\xe0\x38\x86\xa9\xd6\x96\x8c\xa8\xa1\xfb\x78\xc0\x7f\x4e\x40\x1a\x4c\xa3\xbc\x7e\xd9\xdd\xb3\x3f\x90\x98\xb3\x65\x17\xbe\x49\x0f\x88\x17\xd8\x86\x8d\x29\x07\xb0\x8b\x70\x3c\xeb\x94\x0e\x47\xaf\x5f\xbf\x8d\x0e\x44\xd1\xef\xf0\xfa\xb9\x8b\x67\xbe\x04\x00\x27\x70\x8b\x90\x69\x75\x44\x9d\xab\xc7\x42\x6d\x8f\x3b\x8a\x7d\x6b\x77\x2e\x3e\xc3\x79\x04\x6e\x44\x4a\xe5\xd6\x37\x5a\xe7\x88\xa8\x7c\xac\xf5\x0a\x32\xb9\x5a\xa1\x41\x45\x7f\x09\x8b\xc2\xb6\x9f\x08\xc2\x76\x91\xca\x40\x2a\xa8\xb5\x0b\xfe\x28\x7e\xe1\xd5\xf9\x8a\xd0\xfc\x37\xab\x86\x2f\x8c\xda\x1f\xe9\x34\xf2\x33\x16\xd8\xfe\xef\xf1\xfc\xb9\x7d\xb5\x3c\x7d\xca\xa8\x50\xd9\x81\x8c\x2a\x6c\xff\xed\xdd\xbb\x9d\xff\xee\xee\x43\xe9\x1c\xc0\x01\xb6\x88\x7f\xe6\x7f\x06\x00\x00\xff\xff\xf3\xa0\xd4\xb6\xda\x06\x00\x00")

func migrations_testGoBytes() ([]byte, error) {
	return bindataRead(
		_migrations_testGo,
		"migrations_test.go",
	)
}

func migrations_testGo() (*asset, error) {
	bytes, err := migrations_testGoBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "migrations_test.go", size: 1754, mode: os.FileMode(436), modTime: time.Unix(1653682270, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"001_init.down.sql":  _001_initDownSql,
	"001_init.up.sql":    _001_initUpSql,
	"migrations_test.go": migrations_testGo,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"001_init.down.sql":  &bintree{_001_initDownSql, map[string]*bintree{}},
	"001_init.up.sql":    &bintree{_001_initUpSql, map[string]*bintree{}},
	"migrations_test.go": &bintree{migrations_testGo, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
