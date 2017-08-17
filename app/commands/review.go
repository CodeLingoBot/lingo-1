package commands

import (
	"fmt"
	"strings"

	"github.com/juju/errors"

	"github.com/codelingo/flow/backend/service/flow"
	"github.com/codelingo/lingo/vcs"

	"github.com/codelingo/lingo/app/commands/review"
	"github.com/codelingo/lingo/app/util"

	"os"

	"github.com/codegangsta/cli"
	"github.com/codelingo/lingo/app/util/common/config"
)

const (
	vcsGit string = "git"
	vcsP4  string = "perforce"
)

func init() {
	register(&cli.Command{
		Name:        "review",
		Usage:       "Review code following tenets in .lingo.",
		Subcommands: cli.Commands{*pullRequestCmd},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  util.LingoFile.String(),
				Usage: "A .lingo file to perform the review with. If the flag is not set, .lingo files are read from the branch being reviewed.",
			},
			cli.StringFlag{
				Name:  util.DiffFlg.String(),
				Usage: "Review only unstaged changes in the working tree.",
			},
			cli.StringFlag{
				Name:  util.OutputFlg.String(),
				Usage: "File to save found issues to.",
			},
			cli.StringFlag{
				Name:  util.FormatFlg.String(),
				Value: "json-pretty",
				Usage: "How to format the found issues. Possible values are: json, json-pretty.",
			},
			cli.BoolFlag{
				Name:  util.InteractiveFlg.String(),
				Usage: "Be prompted to confirm each issue.",
			},
			cli.StringFlag{
				Name:  util.DirectoryFlg.String(),
				Usage: "Review a given directory.",
			},
			cli.BoolFlag{
				Name:  "debug",
				Usage: "Display debug messages",
			},
			// cli.BoolFlag{
			// 	Name:  "all",
			// 	Usage: "review all files under all directories from pwd down",
			// },
		},
		Description: `
"$ lingo review" will review all code from pwd down.
"$ lingo review <filename>" will only review named file.
`[1:],
		// "$ lingo review" will review any unstaged changes from pwd down.
		// "$ lingo review [<filename>]" will review any unstaged changes in the named files.
		// "$ lingo review --all [<filename>]" will review all code in the named files.
		Action: reviewAction,
	},
		false,
		vcsRq, homeRq, authRq, configRq, versionRq,
	)
}

func reviewAction(ctx *cli.Context) {

	msg, err := reviewCMD(ctx)
	if err != nil {
		if ctx.IsSet("debug") {
			// Debugging
			util.Logger.Debugw("reviewAction", "err_stack", errors.ErrorStack(err))
		}
		util.OSErr(err)
		return
	}

	fmt.Println(msg)
}

func reviewCMD(ctx *cli.Context) (string, error) {
	if ctx.IsSet("debug") {
		defer util.Logger.Sync()
		util.Logger.Debugw("reviewCMD called")
	}
	dir := ctx.String("directory")
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			return "", errors.Trace(err)
		}
	}

	dotlingo, err := review.ReadDotLingo(ctx)
	if err != nil {
		return "", errors.Trace(err)
	}
	vcsType, repo, err := vcs.New()
	if err != nil {
		return "", errors.Trace(err)
	}
	if err := vcs.InitRepo(vcsType, repo); err != nil {
		// TODO(waigani) use error types
		// Note: Prior repo init is a valid state.
		if !strings.Contains(err.Error(), "already exists") {
			return "", errors.Trace(err)
		}
	}

	// TODO: replace this system with nfs-like communication.
	if err = vcs.SyncRepo(vcsType, repo); err != nil {
		return "", errors.Trace(err)
	}

	owner, name, err := repo.OwnerAndNameFromRemote()
	if err != nil {
		return "", errors.Trace(err)
	}

	sha, err := repo.CurrentCommitId()
	if err != nil {
		if noCommitErr(err) {
			return "", errors.New(noCommitErrMsg)
		}

		return "", errors.Trace(err)
	}

	patches, err := repo.Patches()
	if err != nil {
		return "", errors.Trace(err)
	}

	workingDir, err := repo.WorkingDir()
	if err != nil {
		return "", errors.Trace(err)
	}

	cfg, err := config.Platform()
	if err != nil {
		return "", errors.Trace(err)
	}
	vcsTypeStr, err := vcs.TypeToString(vcsType)
	if err != nil {
		return "", errors.Trace(err)
	}
	addr := ""
	issuec := make(chan *flow.Issue)
	errorc := make(chan error)
	switch vcsTypeStr {
	case vcsGit:
		addr, err = cfg.GitServerAddr()
		if err != nil {
			return "", errors.Trace(err)
		}
		hostname, err := cfg.GitRemoteName()
		if err != nil {
			return "", errors.Trace(err)
		}

		issuec, errorc, err = review.RequestReview(&flow.ReviewRequest{
			Host:         addr,
			Hostname:     hostname,
			OwnerOrDepot: &flow.ReviewRequest_Owner{owner},
			Repo:         name,
			Sha:          sha,
			Patches:      patches,
			Vcs:          vcsTypeStr,
			Dir:          workingDir,
			Dotlingo:     dotlingo,
		})

		if err != nil {
			return "", errors.Trace(err)
		}
	case vcsP4:
		addr, err = cfg.P4ServerAddr()
		if err != nil {
			return "", errors.Trace(err)
		}
		hostname, err := cfg.P4RemoteName()
		if err != nil {
			return "", errors.Trace(err)
		}
		depot, err := cfg.P4RemoteDepotName()
		if err != nil {
			return "", errors.Trace(err)
		}
		name = owner + "/" + name
		issuec, errorc, err = review.RequestReview(&flow.ReviewRequest{
			Host:         addr,
			Hostname:     hostname,
			OwnerOrDepot: &flow.ReviewRequest_Depot{depot},
			Repo:         name,
			Sha:          sha,
			Patches:      patches,
			Vcs:          vcsTypeStr,
			Dir:          workingDir,
			Dotlingo:     dotlingo,
		})

		if err != nil {
			return "", errors.Trace(err)
		}
	default:
		return "", errors.New("invalid VCS found.")
	}

	issues, err := review.ConfirmIssues(issuec, errorc, !ctx.Bool("interactive"), ctx.String("output"))
	if err != nil {
		return "", errors.Trace(err)
	}

	if len(issues) == 0 {
		return fmt.Sprintf("Done! No issues found.\n"), nil
	}

	msg, err := review.MakeReport(issues, ctx.String("format"), ctx.String("output"))
	return msg, errors.Trace(err)
}

const noCommitErrMsg = "This looks like a new repository. Please make an initial commit before running `lingo review`. This is only required for the initial commit, subsequent changes to your repo will be picked up by lingo without committing."

// TODO(waigani) use typed error
func noCommitErr(err error) bool {
	return strings.Contains(err.Error(), "ambiguous argument 'HEAD'")
}
