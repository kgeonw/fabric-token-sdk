/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package core

import (
	url2 "net/url"
	"sort"
	"sync"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token/driver"
	"github.com/pkg/errors"
)

var (
	sspDriverMu sync.RWMutex
	sspDriver   = make(map[string]driver.SSPDriver)
)

// RegisterSSPDriver makes an SSPDriver available by the provided name.
// If Register is called twice with the same name or if ssp is nil,
// it panics.
func RegisterSSPDriver(name string, driver driver.SSPDriver) {
	sspDriverMu.Lock()
	defer sspDriverMu.Unlock()
	if driver == nil {
		panic("Register ssp is nil")
	}
	if _, dup := sspDriver[name]; dup {
		panic("Register called twice for ssp " + name)
	}
	sspDriver[name] = driver
}

func unregisterAllSSPDriver() {
	sspDriverMu.Lock()
	defer sspDriverMu.Unlock()
	// For tests.
	sspDriver = make(map[string]driver.SSPDriver)
}

// SSPDrivers returns a sorted list of the names of the registered SSPDriver.
func SSPDrivers() []string {
	sspDriverMu.RLock()
	defer sspDriverMu.RUnlock()
	list := make([]string, 0, len(sspDriver))
	for name := range sspDriver {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

type stateServiceProvider struct {
	sp view.ServiceProvider

	sspsMu sync.RWMutex
	ssps   map[string]driver.StateServiceProvider
}

func NewStateServiceProvider(sp view.ServiceProvider) *stateServiceProvider {
	return &stateServiceProvider{
		sp:   sp,
		ssps: map[string]driver.StateServiceProvider{},
	}
}

func (p *stateServiceProvider) QueryExecutor(url string) (driver.StateQueryExecutor, error) {
	ssp, err := p.ssp(url)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to get ssp for url [%s]", url)
	}
	return ssp.QueryExecutor(url)
}

func (p *stateServiceProvider) Verifier(url string) (driver.StateVerifier, error) {
	ssp, err := p.ssp(url)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to get ssp for url [%s]", url)
	}
	return ssp.Verifier(url)
}

func (p *stateServiceProvider) ssp(url string) (driver.StateServiceProvider, error) {
	p.sspsMu.Lock()
	defer p.sspsMu.Unlock()

	ssp, ok := p.ssps[url]
	if !ok {
		u, err := url2.Parse(url)
		if err != nil {
			return nil, errors.Wrapf(err, "failed parsing url")
		}
		provider, ok := sspDriver[u.Scheme]
		if !ok {
			return nil, errors.Errorf("invalid scheme, expected fabric, got [%s]", u.Scheme)
		}
		ssp, err = provider.New(p.sp)
		if err != nil {
			return nil, errors.Wrapf(err, "failed getting state service provider for [%s]", u.Scheme)
		}
		p.ssps[url] = ssp
	}
	return ssp, nil
}
