package service

import (
	"fmt"
	"io/ioutil"
	"os"

	ssz "github.com/ferranbt/fastssz"
	"github.com/spacemeshos/poet/shared"
)

func persist(filename string, v ssz.Marshaler) error {
	buf, err := v.MarshalSSZ()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, buf, shared.OwnerReadWrite)
	if err != nil {
		return fmt.Errorf("write to disk failure: %v", err)
	}

	return nil

}

func load(filename string, v ssz.Unmarshaler) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file is missing: %v", filename)
		}

		return fmt.Errorf("failed to read file: %v", err)
	}
	return v.UnmarshalSSZ(data)
}
