package render

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/LDTorres/redis-stats/internal/redisstats"
)

func TTLAudit(w io.Writer, audit redisstats.PersistentKeyAudit) {
	fmt.Fprintf(w, "TTL Audit: %s\tDB: %d\n", audit.CapturedAt.Format(time.RFC3339), audit.DB)
	fmt.Fprintf(w, "Scanned keys: %d\tWithout TTL: %d (%.1f%%)\n\n", audit.ScannedKeys, audit.PersistentKeys, audit.PersistentRatio*100)

	if audit.PersistentKeys == 0 {
		fmt.Fprintln(w, "No keys without TTL were found in this DB.")
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Prefix\tKeys Without TTL\tShare\tExample Keys")
	for _, group := range audit.Groups {
		fmt.Fprintf(
			tw,
			"%s\t%d\t%.1f%%\t%s\n",
			group.Prefix,
			group.Count,
			group.Share*100,
			strings.Join(group.ExampleKeys, ", "),
		)
	}
	_ = tw.Flush()
}
