package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/retro-ware/robinson-go/internal/alis"
)

// crashReport prints the VM's crash report and keeps it in saveDir as
// crash-YYYYMMDD-HHMMSS.txt.
func crashReport(err error, saveDir string) {
	var e *alis.Error
	if !errors.As(err, &e) {
		log.Print(err)
		return
	}
	rep := e.Report()
	fmt.Fprint(os.Stderr, rep)
	name := filepath.Join(saveDir, time.Now().Format("crash-20060102-150405.txt"))
	if err := os.WriteFile(name, []byte(rep), 0o644); err != nil {
		log.Print(err)
		return
	}
	log.Printf("%v (report in %s)", err, name)
}
