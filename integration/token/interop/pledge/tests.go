/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pledge

import (
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/integration"
	"github.com/hyperledger-labs/fabric-token-sdk/token"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/interop"
	token2 "github.com/hyperledger-labs/fabric-token-sdk/token/token"
)

func TestAssetTransferWithTwoNetworks(network *integration.Infrastructure) {
	// give some time to the nodes to get the public parameters
	time.Sleep(10 * time.Second)

	alpha := token.TMSID{Network: "alpha"}
	beta := token.TMSID{Network: "beta"}

	alphaURL := interop.FabricURL(alpha)
	betaURL := interop.FabricURL(beta)

	registerAuditor(network, token.WithTMSID(alpha))
	registerAuditor(network, token.WithTMSID(beta))

	tmsIssueCash(network, alpha, "issuerAlpha", "", "USD", 50, "alice")
	tmsIssueCash(network, alpha, "issuerAlpha", "", "EUR", 10, "alice")
	checkBalance(network, "alice", "", "USD", 50)
	checkBalance(network, "alice", "", "EUR", 10)

	// Pledge + Claim
	txid, pledgeid := pledge(network, "alice", "", "USD", 50, "bob", "issuerAlpha", betaURL, time.Minute*1, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 0)
	checkBalance(network, "bob", "", "USD", 0)

	pledgeIDExists(network, "alice", pledgeid, txid, token.WithTMSID(alpha))

	claim(network, "bob", "issuerBeta", &token2.ID{TxId: txid, Index: 0})
	checkBalance(network, "alice", "", "USD", 0)
	checkBalance(network, "bob", "", "USD", 50)

	time.Sleep(time.Minute * 1)
	tmsRedeem(network, "issuerAlpha", &token2.ID{TxId: txid, Index: 0}, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 0)
	checkBalance(network, "bob", "", "USD", 50)

	// Pledge + Reclaim

	tmsIssueCash(network, alpha, "issuerAlpha", "", "USD", 50, "alice")
	checkBalance(network, "alice", "", "USD", 50, token.WithTMSID(alpha))
	txid, _ = pledge(network, "alice", "", "USD", 50, "bob", "issuerAlpha", betaURL, time.Second*10, token.WithTMSID(alpha))

	time.Sleep(time.Second * 15)
	reclaim(network, "alice", "", txid, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 50, token.WithTMSID(alpha))
	checkBalance(network, "bob", "", "USD", 50)

	tmsRedeemWithError(network, "issuerAlpha", &token2.ID{TxId: txid, Index: 0}, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 50, token.WithTMSID(alpha))
	checkBalance(network, "bob", "", "USD", 50)

	scanPledgeIDWithError(network, "alice", pledgeid, txid, []string{"timeout reached"}, token.WithTMSID(alpha))

	// Try to reclaim again

	reclaimWithError(network, "alice", "", txid, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 50, token.WithTMSID(alpha))
	checkBalance(network, "bob", "", "USD", 50)

	// Try to claim after reclaim

	claimWithError(network, "bob", "issuerBeta", &token2.ID{TxId: txid, Index: 0})
	checkBalance(network, "alice", "", "USD", 50, token.WithTMSID(alpha))
	checkBalance(network, "bob", "", "USD", 50)

	// Try to reclaim after claim

	txid, _ = pledge(network, "alice", "", "USD", 10, "bob", "issuerAlpha", betaURL, time.Minute*1, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 40)
	checkBalance(network, "bob", "", "USD", 50)

	claim(network, "bob", "issuerBeta", &token2.ID{TxId: txid, Index: 0})
	checkBalance(network, "alice", "", "USD", 40)
	checkBalance(network, "bob", "", "USD", 60)

	time.Sleep(time.Minute * 1)

	reclaimWithError(network, "alice", "", txid, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 40)
	checkBalance(network, "bob", "", "USD", 60)

	// Try to claim after claim

	txid, pledgeid = pledge(network, "bob", "", "USD", 5, "alice", "issuerBeta", alphaURL, time.Minute*1, token.WithTMSID(beta))
	checkBalance(network, "alice", "", "USD", 40)
	checkBalance(network, "bob", "", "USD", 55)

	pledgeIDExists(network, "bob", pledgeid, txid, token.WithTMSID(beta))

	claim(network, "alice", "issuerAlpha", &token2.ID{TxId: txid, Index: 0})
	checkBalance(network, "alice", "", "USD", 45)
	checkBalance(network, "bob", "", "USD", 55)

	claimWithError(network, "alice", "issuerAlpha", &token2.ID{TxId: txid, Index: 0})
	checkBalance(network, "alice", "", "USD", 45)
	checkBalance(network, "bob", "", "USD", 55)

	time.Sleep(1 * time.Minute)
	tmsRedeem(network, "issuerBeta", &token2.ID{TxId: txid, Index: 0}, token.WithTMSID(beta))
	checkBalance(network, "alice", "", "USD", 45)
	checkBalance(network, "bob", "", "USD", 55)

	// Try to redeem again
	tmsRedeemWithError(network, "issuerBeta", &token2.ID{TxId: txid, Index: 0}, token.WithTMSID(beta), "failed to retrieve pledged token during redeem")
	checkBalance(network, "alice", "", "USD", 45)
	checkBalance(network, "bob", "", "USD", 55)

	// Try to claim or reclaim without pledging

	claimWithError(network, "alice", "issuerAlpha", &token2.ID{TxId: "", Index: 0})
	checkBalance(network, "alice", "", "USD", 45)
	checkBalance(network, "bob", "", "USD", 55)

	reclaimWithError(network, "alice", "", "", token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 45)
	checkBalance(network, "bob", "", "USD", 55)

	// Fast Pledge + Claim
	fastTransferPledgeClaim(network, "alice", "", "USD", 10, "bob", "issuerAlpha", betaURL, time.Minute*1, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 35)
	checkBalance(network, "bob", "", "USD", 65)

	// Fast Pledge + Reclaim
	fastTransferPledgeReclaim(network, "alice", "", "USD", 10, "bob", "issuerAlpha", betaURL, time.Second*5, token.WithTMSID(alpha))
	checkBalance(network, "alice", "", "USD", 35)
	checkBalance(network, "bob", "", "USD", 65)
}
