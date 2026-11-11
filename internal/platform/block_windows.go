package platform

import "os"

func blockDelete(path string) (func(), error) {
	f, err := os.Open(path) // Go opens without FILE_SHARE_DELETE, so the delete fails
	if err != nil {
		return nil, err
	}
	return func() { f.Close() }, nil
}
