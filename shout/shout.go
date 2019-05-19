package shout

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// Auto settings stuff
const (
	noOperation = iota
	toggleLast
	get
)

type autoSettings struct {
	Op   int
	Last bool
}

func startAutoSettings() chan autoSettings {
	settings := make(chan autoSettings)
	go func() {
		curSettings := &autoSettings{}
		for {
			s := <-settings
			switch s.Op {
			case noOperation:
				return
			case toggleLast:
				curSettings.Last = !curSettings.Last
			case get:
				settings <- *curSettings
			}
		}
	}()

	return settings
}

func toggleAutoLast(settings chan autoSettings) {
	settings <- autoSettings{toggleLast, false}
}

func getAutoSettings(settings chan autoSettings) autoSettings {
	settings <- autoSettings{get, false}
	return <-settings
}

// end end auto settings

type icyparseerror struct {
	s string
}

func (ipe *icyparseerror) Error() string {
	return ipe.s
}

func parseIcy(rdr *bufio.Reader, c byte) (string, error) {
	numbytes := int(c) * 16
	bytes := make([]byte, numbytes)
	n, err := io.ReadFull(rdr, bytes)
	if err != nil {
		log.Panic(err)
	}
	if n != numbytes {
		return "", &icyparseerror{"didn't get enough data"} // may be invalid
	}
	return strings.Split(strings.Split(string(bytes), "=")[1], ";")[0], nil
}

func extractMetadata(rdr io.Reader, skip int) <-chan string {
	ch := make(chan string)
	go func() {
		bufrdr := bufio.NewReaderSize(rdr, skip)
		for {
			skipbytes := make([]byte, skip)

			_, err := io.ReadFull(bufrdr, skipbytes)
			if err != nil {
				log.Printf("Failed: %v\n", err)
				close(ch)
				break
			}
			c, err := bufrdr.ReadByte()
			if err != nil {
				log.Panic(err)
			}
			if c > 0 {
				meta, err := parseIcy(bufrdr, c)
				if err != nil {
					log.Panic(err)
				}
				ch <- meta
			}
		}
	}()
	return ch
}

func StreamMeta(url string) {
	log.Printf("Shoutcast stream metadata yanker v0.1\n")
	client := &http.Client{}

	log.Printf("Getting from : %s\n", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	req.Header.Add("Icy-MetaData", "1")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	amount := 0
	if _, err = fmt.Sscan(resp.Header.Get("Icy-Metaint"), &amount); err != nil {
		log.Printf("%v\n", err)
		return
	}

	metaChan := extractMetadata(resp.Body, amount)

	for meta := range metaChan {
		fmt.Println("New meta:")
		fmt.Printf("%s\n", meta)
	}
}

func GetMeta(url string, requestChan chan string) {
	log.Printf("Shoutcast stream metadata yanker v0.5\n")
	client := &http.Client{}

	log.Printf("Getting from : %s\n", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	req.Header.Add("Icy-MetaData", "1")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}

	amount := 0
	if _, err = fmt.Sscan(resp.Header.Get("Icy-Metaint"), &amount); err != nil {
		log.Printf("%v\n", err)
		return
	}

	metaChan := extractMetadata(resp.Body, amount)

	var lastsong string
	settings := startAutoSettings()

	for {
		select {
		case lastsong = <-metaChan:
			if lastsong == "" {
				return
			}

		case request := <-requestChan:
			switch request {
			case "?autolast?":
				toggleAutoLast(settings)
				alonoff := getAutoSettings(settings)
				log.Printf("AUTOLAST: %v\n", alonoff.Last)
			case "?lastsong?":
				log.Printf("Got a request to print the metadata which is: %s\n", lastsong)
			case "":
				log.Printf("Bot died!, we're out too!")
				return
			}
		}
	}
}
