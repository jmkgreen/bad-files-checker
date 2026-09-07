//go:build !linux

package app

func applyNice(int) error {
	return nil
}

func applyIonice(string, int) error {
	return nil
}
