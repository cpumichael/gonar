//go:build !unix

package gonar

func removeAllXattrs(path string) error {
	return nil
}

func lchtimesZero(path string) error {
	return nil
}
