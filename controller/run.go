package controller

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/golang/glog"
)

func (v *VarnishController) Run() error {
	glog.Infof("waiting for initial configuration before starting Varnish")

	v.backend = <-v.backendUpdates
	target, err := os.Create(v.configFile)
	if err != nil {
		return err
	}

	glog.Infof("creating initial VCL config")
	err = v.renderVCL(target, v.backend.Backends, v.backend.Primary)
	if err != nil {
		return err
	}

	cmd, errChan := v.startVarnish()

	v.waitForAdminPort()

	watchErrors := make(chan error)
	go v.watchConfigUpdates(cmd, watchErrors)

	go func() {
		for err := range watchErrors {
			if err != nil {
				glog.Warningf("error while watching for updates: %s", err.Error())
			}
		}
	}()

	return <-errChan
}

func (v *VarnishController) startVarnish() (*exec.Cmd, <-chan error) {
	c := exec.Command(
		"varnishd",
		"-n", v.WorkingDir,
		"-F",
		"-f", v.configFile,
		"-S", v.SecretFile,
		"-s", v.Storage,
		"-a", fmt.Sprintf("%s:%d", v.FrontendAddr, v.FrontendPort),
		"-T", fmt.Sprintf("%s:%d", v.AdminAddr, v.AdminPort),
	)

	c.Dir = "/"
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	r := make(chan error)

	go func() {
		err := c.Run()
		r <- err
	}()

	return c, r
}
