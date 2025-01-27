package s3

import (
	"log"
	"time"
)

func Retry(maxRetries int, backoffBase time.Duration, fn func() error) error {
	var err error
	for i := 0; i <= maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i < maxRetries {
			log.Printf("Retrying (%d/%d) due to error: %v", i+1, maxRetries, err)
			time.Sleep(backoffBase * (1 << i))
		}
	}
	return err
}
