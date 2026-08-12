package generator

import "io/fs"

func fileMode(value uint32) fs.FileMode {
	return fs.FileMode(value)
}
