//go:build windows

package terminal

import (
	"fmt"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

type conptyAPI struct {
	create  *windows.Proc
	resize  *windows.Proc
	release *windows.Proc
	close   *windows.Proc
}

var (
	conptyAPILoadOnce sync.Once
	loadedConPTYAPI   *conptyAPI
	loadedConPTYErr   error
)

func loadConPTYAPI() (*conptyAPI, error) {
	conptyAPILoadOnce.Do(func() {
		path, err := prepareConPTYBundle()
		if err != nil {
			loadedConPTYErr = err
			return
		}
		handle, err := windows.LoadLibraryEx(path, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
		if err != nil {
			loadedConPTYErr = fmt.Errorf("load bundled conpty.dll: %w", err)
			return
		}
		dll := &windows.DLL{Name: path, Handle: handle}
		api := &conptyAPI{}
		api.create, err = findConPTYProcedure(dll, "ConptyCreatePseudoConsole")
		if err == nil {
			api.resize, err = findConPTYProcedure(dll, "ConptyResizePseudoConsole")
		}
		if err == nil {
			api.release, err = findConPTYProcedure(dll, "ConptyReleasePseudoConsole")
		}
		if err == nil {
			api.close, err = findConPTYProcedure(dll, "ConptyClosePseudoConsole")
		}
		if err != nil {
			_ = dll.Release()
			loadedConPTYErr = err
			return
		}
		loadedConPTYAPI = api
	})
	return loadedConPTYAPI, loadedConPTYErr
}

func findConPTYProcedure(dll *windows.DLL, name string) (*windows.Proc, error) {
	procedure, err := dll.FindProc(name)
	if err != nil {
		return nil, fmt.Errorf("find %s in bundled conpty.dll: %w", name, err)
	}
	return procedure, nil
}

func callHRESULT(procedure *windows.Proc, arguments ...uintptr) error {
	result, _, _ := procedure.Call(arguments...)
	if result != 0 {
		return syscall.Errno(result)
	}
	return nil
}
