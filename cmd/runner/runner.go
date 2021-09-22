package main

import (
	"bytes"
	"github.com/spacemeshos/poet/common/utils"
	"github.com/spacemeshos/smutil/log"
	"os"
	"os/exec"
)

func newRunner(args []string) {
	execPath := os.Getenv("EXEC_PATH")
	log.Info("execpath = %s", execPath)
	log.Info("args = %s", args)

	// get filename index under args
	restoreFileNameInd := utils.Contains(args, "--data-paths")
	if restoreFileNameInd != -1 {
		restoreFileName := args[restoreFileNameInd+1]
		args = append(args[:restoreFileNameInd], args[restoreFileNameInd+2:]...)
		stateBucketInd := utils.Contains(args, "--state-bucket")
		if stateBucketInd == -1 {
			log.Panic("state bucket must be supplied")
		}

		stateBucket := args[stateBucketInd+1]
		args = append(args[:stateBucketInd], args[stateBucketInd+2:]...)
		utils.Tarxzf(stateBucket, restoreFileName)
	}

	cmd := exec.Command(execPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Start(); err != nil {
		log.Panic("cmd.Start() failed with '%s'", err)
	}
}

func main() {
	args := os.Args[1:]
	newRunner(args)
	dummyChan := make(chan string)
	<-dummyChan
}
