package lobby

import "syscall"

func init() {
	setpipesz = func(fd uintptr) error {
		// Increase pipe buffer to 1MiB.
		_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETPIPE_SZ, uintptr(pipesz))
		if errno != 0 {
			return errno
		}
		return nil
	}
}
