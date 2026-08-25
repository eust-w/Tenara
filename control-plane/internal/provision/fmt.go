package provision

import "fmt"

func fmtWrapped(out string, err error) error {
	return fmt.Errorf("kubectl apply: %w: %s", err, out)
}
