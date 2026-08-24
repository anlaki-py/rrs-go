//go:build windows && !amd64

package terminal

func bundledConPTYFiles() []conptyBundleFile {
	return nil
}
