/*
Copyright IBM Corp All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package dlog_test

import (
	"runtime"

	"github.com/hyperledger-labs/fabric-smart-client/integration"
	integration2 "github.com/hyperledger-labs/fabric-token-sdk/integration"
	"github.com/hyperledger-labs/fabric-token-sdk/integration/nwo/token"
	"github.com/hyperledger-labs/fabric-token-sdk/integration/token/interop/pledge"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dlog end to end", func() {
	var (
		ii *integration.Infrastructure
	)

	BeforeEach(func() {
		token.Drivers = append(token.Drivers, "dlog")
	})

	AfterEach(func() {
		ii.Stop()
	})

	Describe("Asset Transfer With Two Fabric Networks", func() {
		BeforeEach(func() {
			var err error
			testDir := ""
			if runtime.GOOS == "darwin" {
				testDir = "./testdata"
			}
			ii, err = integration.New(
				integration2.ZKATDLogInteropAssetTransfer.StartPortForNode(),
				testDir,
				pledge.AssetTransferTopology("dlog")...,
			)
			Expect(err).NotTo(HaveOccurred())
			ii.RegisterPlatformFactory(token.NewPlatformFactory())
			ii.Generate()
			ii.Start()
		})

		It("performed a cross network transfer", func() {
			pledge.TestAssetTransferWithTwoNetworks(ii)
		})
	})
})
