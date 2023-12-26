package main

// Read from file, remove sections from each line or replace them with the output of a command.
//
// Original idea is having  a file like
//
// NNNNN foo bar
// NNNNN bar baz
//
// where NNNN is a unix timestamp, and we want to convert that to an actual time using date command
//
// This can be done with this loop but we want to write code so...
//
// while read -a fields
//    do
//      echo "$(date -d@${fields[0]}) ${fields[1]} ${fields[2]}"
// done  < /var/lib/upower/history-rate-5B10W13973-57-4804.dat
//
// Usage:
// cutx -f 1,2 -d X will work exactly like cut.
//
// cutx -f 1='cmd {}',2 will execute cmd `cmd {}` with {} being the column and will replace the column with the output of the command
//

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// TODO allow to pass special characters from CLI
var sepFlag = flag.String("d", "\t", "Separator to use")
var fieldsFlag = flag.String("f", "1-", "Fields")

const endField = math.MaxInt

// Field represents a field, and what we want to do with it
type FieldMapper struct {
	start int
	end   int
	cmd   string
}

func parseFields() []FieldMapper {
	var fields []FieldMapper

	fre, err := regexp.Compile("([0-9-]+)(=(.*))?")
	if err != nil {
		panic(err)
	}
	for _, f := range strings.Split(*fieldsFlag, ",") {
		// this can be either a number, a range, or a number followed by = and a string. If there are quotes around, remove them
		groups := fre.FindStringSubmatch(f)
		if len(groups) == 0 {
			log.Fatalf("Unable to parse -f flag %v: %s does not match regex %s", *fieldsFlag, f, fre)
		}
		// groups[0] is the whole string
		// groups[1] is the index
		// groups[3] is the string

		n := strings.Split(groups[1], "-")
		if len(n) > 2 {
			log.Fatalf("Invalid value for flag -f: %v, %s should either be a number or a single range", *fieldsFlag, groups[1])
		}
		fm := FieldMapper{cmd: groups[3]}
		fm.start, err = strconv.Atoi(n[0])
		fm.start = fm.start - 1
		fm.end = fm.start
		if err != nil {
			log.Fatalf("Unable to convert %v into an integer: %v", n[0], err)
		}
		if len(n) > 1 {
			if n[1] == "" {
				fm.end = endField
			} else {
				// TODO: need to handle the special case where max = "" means the string was N-, which means from here to the end
				fm.end, err = strconv.Atoi(n[1])
				if err != nil {
					log.Fatalf("Unable to convert %v into an integer: %v", n[1], err)
				}
				fm.end = fm.end - 1
			}
		}
		fields = append(fields, fm)
	}
	return fields
}

// runCommand runs command cmd, replacing {} with input string,
// and returns the output.
//
// If cmd is an empty string, runCommand returns input.
func runCommand(cmd, input string) string {
	if cmd == "" {
		return input
	}
	cmd = strings.ReplaceAll(cmd, "{}", input)
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		log.Printf("Error while running command %q: %v", cmd, err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

func main() {
	var fp io.Reader
	var err error
	flag.Parse()

	fms := parseFields()

	fp = os.Stdin
	if flag.NArg() == 1 {
		path := flag.Arg(0)
		fp, err = os.Open(path)
		if err != nil {
			log.Fatalf("Unable to open file %s: %v", path, err)
		}
	}

	r := bufio.NewReader(fp)

	for true {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			log.Fatalf("Error while reading line: %v", err)
		}
		if err == io.EOF {
			break
		}
		// trim trailing newline if it exists
		if len(line) > 1 && line[len(line)-1] == '\n' {
			line = line[:len(line)-2]
		}
		var newline []string
		// split line using separator
		fields := strings.Split(line, *sepFlag)
		// process the fields
		//
		// Loop over fieldMappers, for each one, add to
		// newline whatever needs to be added. Note that we
		// might repeat fields
		for _, fm := range fms {
			for i := fm.start; i <= fm.end && i < len(fields); i++ {
				out := runCommand(fm.cmd, fields[i])
				newline = append(newline, out)
			}
		}
		// we remove the newline at the beginning, so we add it back here
		fmt.Println(strings.Join(newline, *sepFlag))
	}
}
