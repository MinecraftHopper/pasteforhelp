// This is a copy of the FileSystem from PufferPanel
// https://github.com/pufferpanel/pufferpanel/blob/v3/files/filesystem_linux.go
// This was designed and tested to better protect against path traversal or other
// malicious attempts to get files that you should not.
// This however requires the kernel to support openat2 commands.
package main

import (
	"golang.org/x/sys/unix"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type FileServer interface {
	fs.FS

	OpenFile(path string, flags int, mode os.FileMode) (*os.File, error)
	Remove(path string) error

	Close() error
}

type fileServer struct {
	dir  string
	root *os.File
}

func NewFileServer(prefix string) (FileServer, error) {
	f := &fileServer{dir: prefix}
	var err error
	f.root, err = f.resolveRootFd()
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (sfp *fileServer) Prefix() string {
	return sfp.dir
}

func (sfp *fileServer) Open(name string) (fs.File, error) {
	return sfp.OpenFile(name, os.O_RDONLY, 0644)
}

func (sfp *fileServer) resolveRootFd() (*os.File, error) {
	return os.Open(sfp.dir)
}

func (sfp *fileServer) Close() error {
	return sfp.root.Close()
}

func (sfp *fileServer) OpenFile(path string, flags int, mode os.FileMode) (*os.File, error) {
	path = prepPath(path)

	if path == "" {
		return os.Open(sfp.dir)
	}

	//if this is not a create request, nuke mode
	if flags&os.O_CREATE == 0 {
		mode = 0
	}

	var fd int
	var err error
	fd, err = unix.Openat2(getFd(sfp.root), path, &unix.OpenHow{
		Flags:   uint64(flags),
		Mode:    uint64(SyscallMode(mode)),
		Resolve: unix.RESOLVE_BENEATH,
	})
	if err != nil {
		return nil, err
	}

	file := os.NewFile(uintptr(fd), filepath.Base(path))
	return file, err
}

func (sfp *fileServer) Remove(path string) error {
	path = prepPath(path)
	parent := filepath.Dir(path)
	f := filepath.Base(path)

	folder, err := sfp.OpenFile(parent, os.O_RDONLY, 0755)
	if err != nil {
		return err
	}
	defer Close(folder)

	expected, err := sfp.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	stat, err := expected.Stat()
	Close(expected)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return unix.Unlinkat(getFd(folder), f, unix.AT_REMOVEDIR)
	} else {
		return unix.Unlinkat(getFd(folder), f, 0)
	}
}

func getFd(f *os.File) int {
	return int(f.Fd())
}

func prepPath(path string) string {
	path = filepath.Clean(path)
	path = strings.TrimPrefix(path, "/")

	if path == "." || path == "/" {
		return ""
	}

	return path
}

func SyscallMode(i os.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= syscall.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}
	return
}
