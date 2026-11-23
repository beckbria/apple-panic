//go:build js && wasm

package config

import (
	"errors"
	"fmt"
	"os"
	"syscall/js"
)

const defaultWebStorageKey = "apple-panic:config"

func defaultConfigPath() string {
	return defaultWebStorageKey
}

func webStorageKey(path string) string {
	if path == "" {
		return defaultWebStorageKey
	}
	return path
}

func getLocalStorage() (storage js.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("localStorage unavailable: %v", r)
		}
	}()
	storage = js.Global().Get("localStorage")
	if storage.IsUndefined() || storage.IsNull() {
		return js.Undefined(), errors.New("localStorage unavailable")
	}
	return storage, nil
}

func readConfigData(path string) (data []byte, err error) {
	storage, err := getLocalStorage()
	if err != nil {
		return nil, err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("localStorage.getItem failed: %v", r)
		}
	}()
	val := storage.Call("getItem", webStorageKey(path))
	if val.IsNull() || val.IsUndefined() {
		return nil, os.ErrNotExist
	}
	return []byte(val.String()), nil
}

func writeConfigData(path string, data []byte) (err error) {
	storage, err := getLocalStorage()
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("localStorage.setItem failed: %v", r)
		}
	}()
	storage.Call("setItem", webStorageKey(path), string(data))
	return nil
}
