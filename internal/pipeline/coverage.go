package pipeline

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CheckCoverage counts statements, not lines or functions. Floors target the
// artifact builder and source/metadata SDK, rather than inflating CLI coverage.
func CheckCoverage(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	type counts struct{ total, covered int }
	totals := map[string]counts{}
	floors := map[string]int{"github.com/libops/packages/internal/buildapk/": 70, "github.com/libops/packages/buildhelpers/": 70}
	scan := bufio.NewScanner(file)
	if !scan.Scan() || scan.Text() != "mode: set" {
		return fmt.Errorf("expected set coverage profile")
	}
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) != 3 {
			return fmt.Errorf("invalid coverage profile")
		}
		statements, e := strconv.Atoi(fields[1])
		if e != nil || statements < 0 {
			return fmt.Errorf("invalid statement count")
		}
		count, e := strconv.Atoi(fields[2])
		if e != nil || count < 0 {
			return fmt.Errorf("invalid coverage count")
		}
		for prefix := range floors {
			if strings.HasPrefix(fields[0], prefix) {
				c := totals[prefix]
				c.total += statements
				if count > 0 {
					c.covered += statements
				}
				totals[prefix] = c
			}
		}
	}
	if err := scan.Err(); err != nil {
		return err
	}
	for prefix, floor := range floors {
		c := totals[prefix]
		if c.total == 0 || c.covered*100 < c.total*floor {
			return fmt.Errorf("%s coverage %d/%d below %d%%", prefix, c.covered, c.total, floor)
		}
	}
	return nil
}
