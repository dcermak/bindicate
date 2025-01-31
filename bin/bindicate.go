package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/deniswernert/go-fstab"
	cli "github.com/urfave/cli/v3"
)

type Bindicate struct {
	// Prefix path where the bind mounted files are stored
	Prefix string

	Paths []string
}

const (
	bindMountsStart = "# BINDICATE START"
	bindMountEnd    = "# BINDICATE END"
)

func (b Bindicate) createFstabLines() string {
	res := ""
	for _, path := range b.Paths {
		m := fstab.Mount{
			Spec:    filepath.Join(b.Prefix, path),
			File:    path,
			VfsType: "bind",
		}
		res += m.String()
		res += "\n"
	}
	return res
}

func (b Bindicate) SyncBindMountsInFstab(etcFstab string) string {
	lines := strings.Split(etcFstab, "\n")

	newFstab := ""
	inBindicateSection := false

	for _, line := range lines {
		if strings.HasPrefix(line, bindMountsStart) {
			inBindicateSection = true
			continue
		} else if strings.HasPrefix(line, bindMountEnd) {
			inBindicateSection = false
			continue
		}

		if !inBindicateSection {
			if newFstab != "" {
				newFstab += "\n"
			}
			newFstab += line
		}
	}

	if newFstab != "" {
		newFstab += "\n"
	}
	newFstab += bindMountsStart + "\n"
	newFstab += b.createFstabLines()
	newFstab += bindMountEnd

	return newFstab
}

func BindMount(src, dest string) error {
	return syscall.Mount(src, dest, "", syscall.MS_BIND, "")
}

func copyFilePreservePermissions(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	// Create destination, w/ same permissions
	destF, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer destF.Close()

	_, err = io.Copy(destF, srcF)
	return err
}

func (b Bindicate) SetupBindMounts() error {
	errs := make([]error, 0, len(b.Paths))

	for _, path := range b.Paths {
		dest := b.Prefix + "/" + path
		srcDir := filepath.Dir(path)

		_, err := os.Stat(dest)
		if os.IsNotExist(err) {
			// get mode of the src directory
			pStat, statErr := os.Stat(srcDir)
			var mode fs.FileMode
			if statErr != nil {
				mode = pStat.Mode()
			} else {
				mode = 0o0755
			}

			os.MkdirAll(filepath.Dir(dest), mode)

		}

		err = copyFilePreservePermissions(path, dest)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = BindMount(path, dest)
		if err != nil {
			errs = append(errs, err)
			continue
		}

	}
	return errors.Join(errs...)
}

func LoadConfig(configPath string) (*Bindicate, error) {
	conf, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var b Bindicate
	err = json.Unmarshal(conf, &b)
	if err != nil {
		return nil, err
	}

	return &b, nil
}

func main() {
	var confPath []string

	var bindicate *Bindicate

	cmd := cli.Command{
		Name:  "bindicate",
		Usage: "Setup bind mounts from / into a prefix",
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			var err error
			if len(confPath[0]) == 0 {
				confPath[0] = "/var/lib/bindicate/config.json"
			}
			bindicate, err = LoadConfig(confPath[0])

			return ctx, err
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:   "conf-file",
				Min:    0,
				Max:    1,
				Values: &confPath,
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "write-fstab",
				Usage: "Overwrite the current /etc/fstab, appending the bind mounts",
				Action: func(ctx context.Context, c *cli.Command) error {
					contents, err := os.ReadFile("/etc/fstab")
					if err != nil {
						return err
					}

					return os.WriteFile(
						"/etc/fstab",
						[]byte(bindicate.SyncBindMountsInFstab(string(contents))),
						0o0644,
					)
				},
			},
			{
				Name:  "setup-bindmounts",
				Usage: "Sets up the bind mounts into the prefix",
				Action: func(ctx context.Context, c *cli.Command) error {
					return bindicate.SetupBindMounts()
				},
			},
		},
	}

	err := cmd.Run(context.Background(), os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
