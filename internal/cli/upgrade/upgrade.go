package upgrade

import (
	"fmt"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"github.com/tj/go-update"
	"github.com/tj/go/term"
	"github.com/tj/kingpin"

	"github.com/apex/up/internal/cli/root"
	"github.com/apex/up/internal/progressreader"
	"github.com/apex/up/internal/stats"
	"github.com/apex/up/internal/util"
)

func init() {
	cmd := root.Command("upgrade", "Install the latest release of Up.")
	cmd.Action(func(_ *kingpin.ParseContext) error {
		version := root.Cmd.GetVersion()
		start := time.Now()

		defer util.Pad()()
		term.HideCursor()
		defer term.ShowCursor()

		// update polls(1) from tj/gh-polls on github
		p := &update.Project{
			Owner:   "apex",
			Repo:    "up",
			Command: "up",
			Version: version,
		}

		// fetch the new releases
		releases, err := p.LatestReleases()
		if err != nil {
			return errors.Wrap(err, "fetching releases")
		}

		// no updates
		if len(releases) == 0 {
			fmt.Printf("  No updates required, you're good :)\n")
			return nil
		}

		// latest release
		latest := releases[0]

		// find the tarball for this system
		a := latest.FindTarball(runtime.GOOS, runtime.GOARCH)
		if a == nil {
			return errors.Errorf("failed to find a binary for %s %s", runtime.GOOS, runtime.GOARCH)
		}

		// download tarball to a tmp dir
		tarball, err := a.DownloadProxy(progressreader.New)
		if err != nil {
			return errors.Wrap(err, "downloading tarball")
		}

		// install it
		if err := p.Install(tarball); err != nil {
			return errors.Wrap(err, "installing")
		}

		term.ClearAll()
		fmt.Printf("\n  Updated %s to %s :)\n", version, latest.Version)

		stats.Track("Upgrade", map[string]interface{}{
			"from":     version,
			"to":       latest.Version,
			"duration": time.Since(start).Round(time.Millisecond),
		})

		return nil
	})
}
