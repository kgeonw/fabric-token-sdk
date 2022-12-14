/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pledge

import (
	"github.com/hyperledger-labs/fabric-smart-client/integration/nwo/api"
	"github.com/hyperledger-labs/fabric-smart-client/integration/nwo/fabric"
	"github.com/hyperledger-labs/fabric-smart-client/integration/nwo/fsc"
	"github.com/hyperledger-labs/fabric-smart-client/integration/nwo/weaver"
	fabric3 "github.com/hyperledger-labs/fabric-smart-client/platform/fabric/sdk"
	"github.com/hyperledger-labs/fabric-token-sdk/integration/nwo/token"
	fabric2 "github.com/hyperledger-labs/fabric-token-sdk/integration/nwo/token/fabric"
	views2 "github.com/hyperledger-labs/fabric-token-sdk/integration/token/interop/pledge/views"
	pledge2 "github.com/hyperledger-labs/fabric-token-sdk/integration/token/interop/pledge/views/pledge"
	sdk "github.com/hyperledger-labs/fabric-token-sdk/token/sdk"
)

func AssetTransferTopology(tokenSDKDriver string) []api.Topology {
	// Define two Fabric topologies
	f1Topology := fabric.NewTopologyWithName("alpha").SetDefault()
	f1Topology.EnableIdemix()
	f1Topology.AddOrganizationsByName("Org1", "Org2")
	f1Topology.SetNamespaceApproverOrgs("Org1")

	f2Topology := fabric.NewTopologyWithName("beta")
	f2Topology.EnableIdemix()
	f2Topology.AddOrganizationsByName("Org3", "Org4")
	f2Topology.SetNamespaceApproverOrgs("Org3")

	// FSC
	fscTopology := fsc.NewTopology()
	//fscTopology.SetLogging("debug", "")

	wTopology := weaver.NewTopology()
	wTopology.AddRelayServer(f1Topology, "Org1").AddFabricNetwork(f2Topology)
	wTopology.AddRelayServer(f2Topology, "Org3").AddFabricNetwork(f1Topology)

	issuerAlpha := fscTopology.AddNodeByName("issuerAlpha").AddOptions(
		fabric.WithNetworkOrganization("alpha", "Org1"),
		fabric.WithAnonymousIdentity(),
		fabric.WithDefaultNetwork("alpha"),
		token.WithIssuerIdentity("issuer.id1"),
		token.WithDefaultOwnerIdentity(tokenSDKDriver),
	)
	issuerAlpha.RegisterViewFactory("issue", &views2.IssueCashViewFactory{})
	issuerAlpha.RegisterViewFactory("balance", &views2.BalanceViewFactory{})
	issuerAlpha.RegisterViewFactory("transfer.redeem", &pledge2.RedeemViewFactory{})
	issuerAlpha.RegisterResponder(&pledge2.PledgeIssuerResponderView{}, &pledge2.PledgeView{})
	issuerAlpha.RegisterResponder(&pledge2.PledgeIssuerResponderView{}, &pledge2.FastPledgeClaimInitiatorView{})
	issuerAlpha.RegisterResponder(&pledge2.PledgeIssuerResponderView{}, &pledge2.FastPledgeReClaimInitiatorView{})
	issuerAlpha.RegisterResponder(&pledge2.ReclaimIssuerResponderView{}, &pledge2.ReclaimInitiatorView{})
	issuerAlpha.RegisterResponder(&pledge2.ClaimIssuerView{}, &pledge2.ClaimInitiatorView{})
	issuerAlpha.RegisterResponder(&pledge2.ClaimIssuerView{}, &pledge2.FastPledgeClaimResponderView{})
	issuerAlpha.RegisterResponder(&pledge2.ClaimIssuerView{}, &pledge2.FastPledgeReClaimResponderView{})

	issuerBeta := fscTopology.AddNodeByName("issuerBeta").AddOptions(
		fabric.WithNetworkOrganization("beta", "Org3"),
		fabric.WithAnonymousIdentity(),
		fabric.WithDefaultNetwork("beta"),
		token.WithIssuerIdentity("issuer.id2"),
		token.WithDefaultOwnerIdentity(tokenSDKDriver),
	)
	issuerBeta.RegisterViewFactory("issue", &views2.IssueCashViewFactory{})
	issuerBeta.RegisterViewFactory("balance", &views2.BalanceViewFactory{})
	issuerBeta.RegisterViewFactory("transfer.redeem", &pledge2.RedeemViewFactory{})
	issuerBeta.RegisterResponder(&pledge2.PledgeIssuerResponderView{}, &pledge2.PledgeView{})
	issuerBeta.RegisterResponder(&pledge2.PledgeIssuerResponderView{}, &pledge2.FastPledgeClaimInitiatorView{})
	issuerBeta.RegisterResponder(&pledge2.PledgeIssuerResponderView{}, &pledge2.FastPledgeReClaimInitiatorView{})
	issuerBeta.RegisterResponder(&pledge2.ReclaimIssuerResponderView{}, &pledge2.ReclaimInitiatorView{})
	issuerBeta.RegisterResponder(&pledge2.ClaimIssuerView{}, &pledge2.ClaimInitiatorView{})
	issuerBeta.RegisterResponder(&pledge2.ClaimIssuerView{}, &pledge2.FastPledgeClaimResponderView{})
	issuerBeta.RegisterResponder(&pledge2.ClaimIssuerView{}, &pledge2.FastPledgeReClaimResponderView{})

	auditor := fscTopology.AddNodeByName("auditor").AddOptions(
		fabric.WithNetworkOrganization("alpha", "Org1"),
		fabric.WithNetworkOrganization("beta", "Org3"),
		fabric.WithAnonymousIdentity(),
		token.WithAuditorIdentity(),
	)
	auditor.RegisterViewFactory("register", &views2.RegisterAuditorViewFactory{})
	auditor.RegisterViewFactory("balance", &views2.BalanceViewFactory{})

	alice := fscTopology.AddNodeByName("alice").AddOptions(
		fabric.WithNetworkOrganization("alpha", "Org2"),
		fabric.WithAnonymousIdentity(),
		fabric.WithDefaultNetwork("alpha"),
		token.WithOwnerIdentity(tokenSDKDriver, "alice.id1"),
	)
	alice.RegisterResponder(&views2.AcceptCashView{}, &views2.IssueCashView{})
	alice.RegisterViewFactory("balance", &views2.BalanceViewFactory{})
	alice.RegisterViewFactory("transfer.claim", &pledge2.ClaimInitiatorViewFactory{})
	alice.RegisterViewFactory("transfer.pledge", &pledge2.PledgeViewFactory{})
	alice.RegisterViewFactory("transfer.reclaim", &pledge2.ReclaimViewFactory{})
	alice.RegisterViewFactory("transfer.fastTransfer", &pledge2.FastPledgeClaimInitiatorViewFactory{})
	alice.RegisterViewFactory("transfer.fastPledgeReclaim", &pledge2.FastPledgeReClaimInitiatorViewFactory{})
	alice.RegisterViewFactory("transfer.scan", &pledge2.ScanViewFactory{})
	alice.RegisterResponder(&pledge2.PledgeRecipientResponderView{}, &pledge2.PledgeView{})
	alice.RegisterResponder(&pledge2.FastPledgeClaimResponderView{}, &pledge2.FastPledgeClaimInitiatorView{})
	alice.RegisterResponder(&pledge2.FastPledgeReClaimResponderView{}, &pledge2.FastPledgeReClaimInitiatorView{})

	bob := fscTopology.AddNodeByName("bob").AddOptions(
		fabric.WithNetworkOrganization("beta", "Org4"),
		fabric.WithAnonymousIdentity(),
		fabric.WithDefaultNetwork("beta"),
		token.WithOwnerIdentity(tokenSDKDriver, "bob.id1"),
	)
	bob.RegisterResponder(&views2.AcceptCashView{}, &views2.IssueCashView{})
	bob.RegisterViewFactory("balance", &views2.BalanceViewFactory{})
	bob.RegisterViewFactory("transfer.claim", &pledge2.ClaimInitiatorViewFactory{})
	bob.RegisterViewFactory("transfer.pledge", &pledge2.PledgeViewFactory{})
	bob.RegisterViewFactory("transfer.reclaim", &pledge2.ReclaimViewFactory{})
	bob.RegisterViewFactory("transfer.fastTransfer", &pledge2.FastPledgeClaimInitiatorViewFactory{})
	bob.RegisterViewFactory("transfer.fastPledgeReclaim", &pledge2.FastPledgeReClaimInitiatorViewFactory{})
	bob.RegisterViewFactory("transfer.scan", &pledge2.ScanViewFactory{})
	bob.RegisterResponder(&pledge2.PledgeRecipientResponderView{}, &pledge2.PledgeView{})
	bob.RegisterResponder(&pledge2.FastPledgeClaimResponderView{}, &pledge2.FastPledgeClaimInitiatorView{})
	bob.RegisterResponder(&pledge2.FastPledgeReClaimResponderView{}, &pledge2.FastPledgeReClaimInitiatorView{})

	tokenTopology := token.NewTopology()
	tokenTopology.SetSDK(fscTopology, &sdk.SDK{})
	tms := tokenTopology.AddTMS(fscTopology.ListNodes(), f1Topology, f1Topology.Channels[0].Name, tokenSDKDriver)
	tms.SetTokenGenPublicParams("100", "2")
	fabric2.SetOrgs(tms, "Org1")
	tms.AddAuditor(auditor)

	tms = tokenTopology.AddTMS(fscTopology.ListNodes(), f2Topology, f2Topology.Channels[0].Name, tokenSDKDriver)
	tms.SetTokenGenPublicParams("100", "2")
	fabric2.SetOrgs(tms, "Org3")
	tms.AddAuditor(auditor)

	// Add Fabric SDK to FSC Nodes
	fscTopology.AddSDK(&fabric3.SDK{})

	return []api.Topology{f1Topology, f2Topology, tokenTopology, wTopology, fscTopology}
}
