package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/AspieSoft/goutil/v5"
	"gopkg.in/yaml.v2"
)

type diskConfig struct {
	Table string
	Unit string

	Partitions []map[string]*DiskPart
}

type DiskPart struct {
	Part string
	Size int
	Flags []string
	Boot bool

	start int
	end int
}

func main(){
	var file string = "disk.yml"

	partSize := map[string]int{
		"root": 16000,
		"var": -1,
	}

	args := goutil.MapArgs()

	if args["file"] != "" {
		file = args["file"]
	}else if args["0"] != "" {
		file = args["0"]
	}

	if args["rootSize"] != "" {
		if i, err := strconv.Atoi(args["rootSize"]); err == nil && i > 0 {
			partSize["root"] = i
		}
	}

	if args["varSize"] != "" {
		if i, err := strconv.Atoi(args["varSize"]); err == nil && i > 0 {
			partSize["var"] = i
		}
	}

	if args["ramSize"] != "" {
		if i, err := strconv.Atoi(args["ramSize"]); err == nil && i > 0 {
			partSize["swap"] = i
		}
	}

	if args["gameSize"] != "" {
		if i, err := strconv.Atoi(args["gameSize"]); err == nil && i > 0 {
			partSize["games"] = i
		}
	}

	if args["winSize"] != "" {
		if i, err := strconv.Atoi(args["winSize"]); err == nil && i > 0 {
			partSize["windows"] = i
		}
	}

	buf, err := os.ReadFile(file)
	if err != nil {
		panic(err)
	}

	config := diskConfig{}
	err = yaml.Unmarshal(buf, &config)
	if err != nil {
		panic(err)
	}

	diskPos := 1
	fromEnd := false
	for _, partData := range config.Partitions {
		for name, part := range partData {
			size := part.Size
			if v, ok := partSize[name]; ok {
				size = v
			}

			if size == -1 {
				fromEnd = true
				part.start = diskPos
				part.end = -1

				for _, pd := range config.Partitions {
					for n, p := range pd {
						s := p.Size
						if s >= 0 {
							if v, ok := partSize[n]; ok {
								s = v
							}
						}

						if s > 0 {
							part.end -= s
						}
					}
				}

				part.Size = -2
				diskPos = part.end
				continue
			}else if size == 0 {
				part.Size = -3
				continue
			}

			if fromEnd {
				part.start = diskPos
				diskPos += size

				if diskPos > -1 {
					part.Size = -3
					continue
				}

				part.end = diskPos
				part.Size = -2
				continue
			}

			part.start = diskPos
			part.end = diskPos + size
			part.Size = -2
			diskPos += size
		}
	}

	res := "mktable "+config.Table+"\n"
	res += "unit "+config.Unit+"\n"

	i := 1
	bootPar := 0
	for _, partData := range config.Partitions {
		for name, part := range partData {
			if part.Size == -3 {
				continue
			}

			res += "mkpart "+part.Part+" "+strconv.Itoa(part.start)+" "+strconv.Itoa(part.end)+"\n"
			res += "name "+strconv.Itoa(i)+" "+name+"\n"

			for _, flag := range part.Flags {
				res += "set "+strconv.Itoa(i)+" "+flag+" on\n"
			}

			if part.Boot && bootPar == 0 {
				bootPar = i
			}

			i++
		}
	}

	if bootPar != 0 {
		res += "set "+strconv.Itoa(bootPar)+" boot on\n"
	}

	res += "quit\n"

	fmt.Println(res)
}
