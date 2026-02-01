package tailer

import (
	"github.com/nxadm/tail"
	"log"
)

// TailFile tails a file and sends lines to the provided channel
func TailFile(path string, lines chan<- string) {
	t, err := tail.TailFile(path, tail.Config{
		Follow: true,
		ReOpen: true,
		// If file doesn't exist, we still want to wait for it to appear
		MustExist: false, 
		Poll:      true, // Polling is often safer in Docker mounts
	})
	if err != nil {
		log.Printf("Error tailing file %s: %v", path, err)
		return
	}

	go func() {
		for line := range t.Lines {
			if line.Err != nil {
				log.Printf("Error reading line from %s: %v", path, line.Err)
				continue
			}
			lines <- line.Text
		}
	}()
}
