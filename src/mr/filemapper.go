package mr

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type FileMapper struct {
	MapId int
	files map[int]*os.File
}

func (fm *FileMapper) WriteKv(key int, kv *KeyValue) error {
	file, err := fm.getFile(key)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	err = enc.Encode(&kv)
	if err != nil {
		return fmt.Errorf("cannot encode: %w", err)
	}
	return nil
}

func (fm *FileMapper) getFile(key int) (*os.File, error) {
	if file, exist := fm.files[key]; exist {
		return file, nil
	} else {
		file, err := ioutil.TempFile("", fmt.Sprintf("mr-%d-%d.json", fm.MapId, key))
		if err != nil {
			return nil, err
		}
		fm.files[key] = file
		return file, nil
	}
}

func (fm *FileMapper) Commit() error {
	for key, file := range fm.files {
		err := file.Close()
		if err != nil {
			return fmt.Errorf("could not close file %s: %w", file.Name(), err)
		}
		rename := fmt.Sprintf("mr-%d-%d.json", fm.MapId, key)
		err = os.Rename(file.Name(), rename)
		if err != nil {
			return fmt.Errorf("could not rename file %s to %s: %w",
				file.Name(),
				rename,
				err)
		}
	}
	return nil
}
