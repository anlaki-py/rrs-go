//go:build windows && amd64

package terminal

import _ "embed"

var (
	//go:embed conpty_assets/windows_amd64/conpty.dll
	bundledConPTYDLL []byte

	//go:embed conpty_assets/windows_amd64/x64/OpenConsole.bin
	bundledOpenConsoleX64 []byte

	//go:embed conpty_assets/windows_amd64/arm64/OpenConsole.bin
	bundledOpenConsoleARM64 []byte
)

func bundledConPTYFiles() []conptyBundleFile {
	return []conptyBundleFile{
		{path: "conpty.dll", content: bundledConPTYDLL},
		{path: "x64/OpenConsole.exe", content: bundledOpenConsoleX64},
		{path: "arm64/OpenConsole.exe", content: bundledOpenConsoleARM64},
	}
}
