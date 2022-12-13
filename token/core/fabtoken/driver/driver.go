/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package driver

import (
	fabric2 "github.com/hyperledger-labs/fabric-smart-client/platform/fabric"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view/services/kvs"
	"github.com/hyperledger-labs/fabric-token-sdk/token"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/config"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/fabric"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/fabtoken"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/fabtoken/ppm"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/identity"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/identity/msp"
	"github.com/hyperledger-labs/fabric-token-sdk/token/driver"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/network"
	"github.com/pkg/errors"
)

type Driver struct {
}

func (d *Driver) NewStateQueryExecutor(sp driver.ServiceProvider, url string) (driver.StateQueryExecutor, error) {
	return &fabtoken.StateQueryExecutor{
		TargetNetworkURL: url,
		SP:               sp,
		RelaySelector:    fabric2.GetDefaultFNS(sp),
	}, nil
}

func (d *Driver) NewStateVerifier(sp driver.ServiceProvider, url string) (driver.StateVerifier, error) {
	return &fabtoken.StateVerifier{
		NetworkURL:    url,
		SP:            sp,
		RelaySelector: fabric2.GetDefaultFNS(sp),
	}, nil
}

func (d *Driver) PublicParametersFromBytes(params []byte) (driver.PublicParameters, error) {
	pp, err := fabtoken.NewPublicParamsFromBytes(params, fabtoken.PublicParameters)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal public parameters")
	}
	return pp, nil
}

func (d *Driver) NewTokenService(sp view.ServiceProvider, publicParamsFetcher driver.PublicParamsFetcher, networkID string, channel string, namespace string) (driver.TokenManagerService, error) {
	n := network.GetInstance(sp, networkID, channel)
	if n == nil {
		return nil, errors.Errorf("network [%s] does not exists", networkID)
	}
	v, err := n.Vault(namespace)
	if err != nil {
		return nil, errors.WithMessagef(err, "vault [%s:%s] does not exists", networkID, namespace)
	}
	qe := v.TokenVault().QueryEngine()
	networkLocalMembership := n.LocalMembership()

	tmsConfig, err := config.NewTokenSDK(view.GetConfigService(sp)).GetTMS(networkID, channel, namespace)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create config manager")
	}

	// Prepare wallets
	wallets := identity.NewWallets()
	mspWalletFactory := msp.NewWalletFactory(
		sp,        // service provider
		networkID, // network ID
		tmsConfig, // config manager
		view.GetIdentityProvider(sp).DefaultIdentity(), // FSC identity
		networkLocalMembership.DefaultIdentity(),       // network default identity
		msp.NewSigService(view.GetSigService(sp)),      // signer service
		view.GetEndpointService(sp),                    // endpoint service
	)
	wallet, err := mspWalletFactory.NewX509Wallet(driver.OwnerRole)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create owner wallet")
	}
	wallets.Put(driver.OwnerRole, wallet)
	wallet, err = mspWalletFactory.NewX509Wallet(driver.IssuerRole)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create issuer wallet")
	}
	wallets.Put(driver.IssuerRole, wallet)
	wallet, err = mspWalletFactory.NewX509Wallet(driver.AuditorRole)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create auditor wallet")
	}
	wallets.Put(driver.AuditorRole, wallet)
	wallet, err = mspWalletFactory.NewX509Wallet(driver.CertifierRole)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create certifier wallet")
	}
	wallets.Put(driver.CertifierRole, wallet)

	// Instantiate the token service
	tmsID := token.TMSID{
		Network:   networkID,
		Channel:   channel,
		Namespace: namespace,
	}
	service := fabtoken.NewService(
		sp,
		tmsID,
		ppm.NewPublicParamsManager(
			fabtoken.PublicParameters,
			qe,
			&fabtoken.PublicParamsLoader{
				PublicParamsFetcher: publicParamsFetcher,
				PPLabel:             fabtoken.PublicParameters,
			},
		),
		&fabtoken.VaultTokenLoader{TokenVault: qe},
		qe,
		identity.NewProvider(sp, fabtoken.NewEnrollmentIDDeserializer(), wallets),
		fabtoken.NewDeserializer(),
		tmsConfig,
		kvs.GetService(sp),
	)
	if err := service.PPM.Load(); err != nil {
		return nil, errors.WithMessage(err, "failed to update public parameters")
	}
	return service, nil
}

func (d *Driver) NewValidator(params driver.PublicParameters) (driver.Validator, error) {
	pp, ok := params.(*fabtoken.PublicParams)
	if !ok {
		return nil, errors.Errorf("invalid public parameters type [%T]", params)
	}
	return fabtoken.NewValidator(pp, fabtoken.NewDeserializer())
}

func (d *Driver) NewPublicParametersManager(params driver.PublicParameters) (driver.PublicParamsManager, error) {
	pp, ok := params.(*fabtoken.PublicParams)
	if !ok {
		return nil, errors.Errorf("invalid public parameters type [%T]", params)
	}
	return ppm.NewPublicParamsManagerFromParams(pp)
}

func init() {
	d := &Driver{}
	core.Register(fabtoken.PublicParameters, d)
	fabric.RegisterStateDriver(fabtoken.PublicParameters, d)
}
