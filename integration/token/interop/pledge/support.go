/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pledge

import (
	"encoding/json"
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/integration"
	"github.com/hyperledger-labs/fabric-smart-client/integration/nwo/common"
	"github.com/hyperledger-labs/fabric-smart-client/pkg/api"
	pledge2 "github.com/hyperledger-labs/fabric-token-sdk/integration/token/interop/pledge/views/pledge"
	views2 "github.com/hyperledger-labs/fabric-token-sdk/integration/token/interop/views"
	"github.com/hyperledger-labs/fabric-token-sdk/token"
	token2 "github.com/hyperledger-labs/fabric-token-sdk/token/token"
	. "github.com/onsi/gomega"
)

func registerAuditor(network *integration.Infrastructure, opts ...token.ServiceOption) {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())

	_, err = network.Client("auditor").CallView("register", common.JSONMarshall(&views2.RegisterAuditor{
		TMSID: options.TMSID(),
	}))
	Expect(err).NotTo(HaveOccurred())
}

func tmsIssueCash(network *integration.Infrastructure, tmsID token.TMSID, issuer string, wallet string, typ string, amount uint64, receiver string) string {
	txid, err := network.Client(issuer).CallView("issue", common.JSONMarshall(&views2.IssueCash{
		TMSID:        tmsID,
		IssuerWallet: wallet,
		TokenType:    typ,
		Quantity:     amount,
		Recipient:    network.Identity(receiver),
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(network.Client(receiver).IsTxFinal(
		common.JSONUnmarshalString(txid),
		api.WithNetwork(tmsID.Network),
		api.WithChannel(tmsID.Channel),
	)).NotTo(HaveOccurred())
	Expect(network.Client("auditor").IsTxFinal(
		common.JSONUnmarshalString(txid),
		api.WithNetwork(tmsID.Network),
		api.WithChannel(tmsID.Channel),
	)).NotTo(HaveOccurred())

	return common.JSONUnmarshalString(txid)
}

func checkBalance(network *integration.Infrastructure, id string, wallet string, typ string, expected uint64, opts ...token.ServiceOption) {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())
	res, err := network.Client(id).CallView("balance", common.JSONMarshall(&views2.Balance{
		Wallet: wallet,
		Type:   typ,
		TMSID: token.TMSID{
			Network:   options.Network,
			Channel:   options.Channel,
			Namespace: options.Namespace,
		},
	}))
	Expect(err).NotTo(HaveOccurred())
	b := &views2.BalanceResult{}
	common.JSONUnmarshal(res.([]byte), b)
	Expect(b.Type).To(BeEquivalentTo(typ))
	q, err := token2.ToQuantity(b.Quantity, 64)
	Expect(err).NotTo(HaveOccurred())
	expectedQ := token2.NewQuantityFromUInt64(expected)
	Expect(expectedQ.Cmp(q)).To(BeEquivalentTo(0), "[%s]!=[%s]", expected, q)
}

func pledge(network *integration.Infrastructure, sender, wallet, typ string, amount uint64, receiver, issuer, destNetwork string, deadline time.Duration, opts ...token.ServiceOption) (string, string) {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())
	raw, err := network.Client(sender).CallView("transfer.pledge", common.JSONMarshall(&pledge2.Pledge{
		OriginTMSID:           options.TMSID(),
		Amount:                amount,
		ReclamationDeadline:   deadline,
		Type:                  typ,
		DestinationNetworkURL: destNetwork,
		Issuer:                network.Identity(issuer),
		Recipient:             network.Identity(receiver),
		OriginWallet:          wallet,
	}))
	Expect(err).NotTo(HaveOccurred())
	info := &pledge2.PledgeInformation{}
	err = json.Unmarshal(raw.([]byte), info)
	Expect(err).NotTo(HaveOccurred())
	Expect(network.Client(sender).IsTxFinal(
		info.TxID,
		api.WithNetwork(options.TMSID().Network),
		api.WithChannel(options.TMSID().Channel),
	)).NotTo(HaveOccurred())

	return info.TxID, info.PledgeID
}

func pledgeIDExists(network *integration.Infrastructure, id, pledgeid string, startingTransactionID string, opts ...token.ServiceOption) {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())
	raw, err := network.Client(id).CallView("transfer.scan", common.JSONMarshall(&pledge2.Scan{
		TMSID:                 options.TMSID(),
		Timeout:               120 * time.Second,
		PledgeID:              pledgeid,
		StartingTransactionID: startingTransactionID,
	}))
	Expect(err).NotTo(HaveOccurred())
	var res bool
	err = json.Unmarshal(raw.([]byte), &res)
	Expect(err).NotTo(HaveOccurred())
	Expect(res).Should(BeTrue())
}

func scanPledgeIDWithError(network *integration.Infrastructure, id, pledgeid string, startingTransactionID string, errorMsgs []string, opts ...token.ServiceOption) {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())
	_, err = network.Client(id).CallView("transfer.scan", common.JSONMarshall(&pledge2.Scan{
		TMSID:                 options.TMSID(),
		Timeout:               120 * time.Second,
		PledgeID:              pledgeid,
		StartingTransactionID: startingTransactionID,
	}))
	Expect(err).To(HaveOccurred())
	for _, msg := range errorMsgs {
		Expect(err.Error()).To(ContainSubstring(msg))
	}
}

func reclaim(network *integration.Infrastructure, sender string, wallet string, txid string, opts ...token.ServiceOption) string {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())
	txID, err := network.Client(sender).CallView("transfer.reclaim", common.JSONMarshall(&pledge2.Reclaim{ // TokenID contains the identifier of the token to be reclaimed.
		TokenID:  &token2.ID{TxId: txid, Index: 0},
		WalletID: wallet,
		TMSID:    options.TMSID(),
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(network.Client("alice").IsTxFinal(common.JSONUnmarshalString(txID))).NotTo(HaveOccurred())

	return common.JSONUnmarshalString(txID)
}

func reclaimWithError(network *integration.Infrastructure, sender string, wallet string, txid string, opts ...token.ServiceOption) {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())
	_, err = network.Client(sender).CallView("transfer.reclaim", common.JSONMarshall(&pledge2.Reclaim{ // TokenID contains the identifier of the token to be reclaimed.
		TokenID:  &token2.ID{TxId: txid, Index: 0},
		WalletID: wallet,
		TMSID:    options.TMSID(),
	}))
	Expect(err).To(HaveOccurred())
}

func claim(network *integration.Infrastructure, recipient string, issuer string, originTokenID *token2.ID) string {
	txid, err := network.Client(recipient).CallView("transfer.claim", common.JSONMarshall(&pledge2.Claim{
		OriginTokenID: originTokenID,
		Issuer:        issuer,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(network.Client(recipient).IsTxFinal(common.JSONUnmarshalString(txid))).NotTo(HaveOccurred())

	return common.JSONUnmarshalString(txid)
}

func claimWithError(network *integration.Infrastructure, recipient string, issuer string, originTokenID *token2.ID) {
	_, err := network.Client(recipient).CallView("transfer.claim", common.JSONMarshall(&pledge2.Claim{
		OriginTokenID: originTokenID,
		Issuer:        issuer,
	}))
	Expect(err).To(HaveOccurred())
}

func tmsRedeem(network *integration.Infrastructure, issuer string, tokenID *token2.ID, opts ...token.ServiceOption) string {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())
	txID, err := network.Client(issuer).CallView("transfer.redeem", common.JSONMarshall(&pledge2.Redeem{
		TokenID: tokenID,
		TMSID:   options.TMSID(),
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(network.Client(issuer).IsTxFinal(common.JSONUnmarshalString(txID))).NotTo(HaveOccurred())

	return common.JSONUnmarshalString(txID)
}

func tmsRedeemWithError(network *integration.Infrastructure, issuer string, tokenID *token2.ID, opt token.ServiceOption, errorMsgs ...string) {
	options, err := token.CompileServiceOptions(opt)
	Expect(err).NotTo(HaveOccurred())
	_, err = network.Client(issuer).CallView("transfer.redeem", common.JSONMarshall(&pledge2.Redeem{
		TokenID: tokenID,
		TMSID:   options.TMSID(),
	}))
	Expect(err).To(HaveOccurred())
	for _, msg := range errorMsgs {
		Expect(err.Error()).To(ContainSubstring(msg))
	}
}

func fastTransferPledgeClaim(network *integration.Infrastructure, sender, wallet, typ string, amount uint64, receiver, issuer, destNetwork string, deadline time.Duration, opts ...token.ServiceOption) {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())

	_, err = network.Client(sender).CallView("transfer.fastTransfer", common.JSONMarshall(&pledge2.FastPledgeClaim{
		OriginTMSID:           options.TMSID(),
		Amount:                amount,
		ReclamationDeadline:   deadline,
		Type:                  typ,
		DestinationNetworkURL: destNetwork,
		Issuer:                network.Identity(issuer),
		Recipient:             network.Identity(receiver),
		OriginWallet:          wallet,
	}))
	Expect(err).NotTo(HaveOccurred())
}

func fastTransferPledgeReclaim(network *integration.Infrastructure, sender, wallet, typ string, amount uint64, receiver, issuer, destNetwork string, deadline time.Duration, opts ...token.ServiceOption) {
	options, err := token.CompileServiceOptions(opts...)
	Expect(err).NotTo(HaveOccurred())

	_, err = network.Client(sender).CallView("transfer.fastPledgeReclaim", common.JSONMarshall(&pledge2.FastPledgeReClaim{
		OriginTMSID:           options.TMSID(),
		Amount:                amount,
		ReclamationDeadline:   deadline,
		Type:                  typ,
		DestinationNetworkURL: destNetwork,
		Issuer:                network.Identity(issuer),
		Recipient:             network.Identity(receiver),
		OriginWallet:          wallet,
	}))
	Expect(err).NotTo(HaveOccurred())
}
