/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pledge

import (
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"time"

	view2 "github.com/hyperledger-labs/fabric-smart-client/platform/view"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view/services/assert"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/interop/pledge"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/ttx"
	"github.com/pkg/errors"
)

// Pledge contains the input information for a transfer
type Pledge struct {
	// OriginTMSID identifies the TMS to use to perform the token operation.
	OriginTMSID token.TMSID
	// OriginWallet is the identifier of the wallet that owns the tokens to transfer in the origin network
	OriginWallet string
	// Type of tokens to transfer
	Type string
	// Amount to transfer
	Amount uint64
	// Issuer is the identity of the issuer's FSC node
	Issuer view.Identity
	// Recipient is the identity of the recipient's FSC node
	Recipient view.Identity
	// DestinationNetworkURL is the destination network's url to transfer the token to
	DestinationNetworkURL string
	// ReclamationDeadline is the time after which we can reclaim the funds in case they were not transferred
	ReclamationDeadline time.Duration
	// PlegdeID is the unique identifier of the pledge
	PledgeID string
}

// PledgeView is the view of the initiator of a pledge operation
type PledgeView struct {
	*Pledge
}

type PledgeInformation struct {
	TxID     string
	PledgeID string
}

func (pv *PledgeView) Call(context view.Context) (interface{}, error) {
	// Collect recipient's token-sdk identity
	recipient, err := pledge.RequestPledgeRecipientIdentity(context, pv.Recipient, pv.DestinationNetworkURL, token.WithTMSID(pv.OriginTMSID))
	assert.NoError(err, "failed getting recipient identity")

	// Collect issuer's token-sdk identity
	// TODO: shall we ask for the issuer identity here and not the owner identity?
	issuer, err := pledge.RequestRecipientIdentity(context, pv.Issuer, token.WithTMSID(pv.OriginTMSID))
	assert.NoError(err, "failed getting recipient identity")

	// Create a new transaction
	tx, err := pledge.NewAnonymousTransaction(
		context,
		ttx.WithAuditor(view2.GetIdentityProvider(context).Identity("auditor")),
		ttx.WithTMSID(pv.OriginTMSID),
	)
	assert.NoError(err, "failed created a new transaction")

	// The sender will select tokens owned by this wallet
	senderWallet := pledge.GetWallet(context, pv.OriginWallet, token.WithTMSID(pv.OriginTMSID))
	assert.NotNil(senderWallet, "sender wallet [%s] not found", pv.OriginWallet)

	pv.PledgeID, err = generatePledgeID()
	assert.NoError(err, "failed to generate pledge ID")

	err = tx.Pledge(senderWallet, pv.DestinationNetworkURL, pv.ReclamationDeadline, recipient, issuer, pv.PledgeID, pv.Type, pv.Amount)
	assert.NoError(err, "failed pledging")

	// Collect signatures
	_, err = context.RunView(pledge.NewCollectEndorsementsView(tx))
	assert.NoError(err, "failed to sign pledge transaction")

	// Last but not least, the issuer sends the transaction for ordering and waits for transaction finality.
	_, err = context.RunView(pledge.NewOrderingAndFinalityView(tx))
	assert.NoError(err, "failed to commit issue transaction")

	// Inform the recipient of the pledge,
	// recall that the recipient might be aware of only the other network
	_, err = context.RunView(pledge.NewDistributePledgeView(tx))
	assert.NoError(err, "failed to send the pledge info")

	return json.Marshal(&PledgeInformation{TxID: tx.ID(), PledgeID: pv.PledgeID})
}

type PledgeViewFactory struct{}

func (pvf *PledgeViewFactory) NewView(in []byte) (view.View, error) {
	f := &PledgeView{Pledge: &Pledge{}}
	err := json.Unmarshal(in, f.Pledge)
	assert.NoError(err, "failed unmarshalling input")

	return f, nil
}

type PledgeRecipientResponderView struct{}

func (p *PledgeRecipientResponderView) Call(context view.Context) (interface{}, error) {
	me, err := pledge.RespondRequestPledgeRecipientIdentity(context)
	assert.NoError(err, "failed to respond to identity request")

	// At some point, the recipient receives the pledge info
	pledgeInfo, err := pledge.ReceivePledgeInfo(context)
	assert.NoError(err, "failed to receive pledge info")

	// Perform any check that is needed to validate the pledge.
	logger.Debugf("The pledge info is %v", pledgeInfo)
	assert.Equal(me, pledgeInfo.Script.Recipient, "recipient is different [%s]!=[%s]", me, pledgeInfo.Script.Recipient)

	// TODO: check pledgeInfo.Script.DestinationNetwork

	// Store the pledge and send a notification back
	_, err = context.RunView(pledge.NewAcceptPledgeView(pledgeInfo))
	assert.NoError(err, "failed accepting pledge info")

	return nil, nil
}

type PledgeIssuerResponderView struct{}

func (p *PledgeIssuerResponderView) Call(context view.Context) (interface{}, error) {
	me, err := pledge.RespondRequestRecipientIdentity(context)
	assert.NoError(err, "failed to respond to identity request")

	// At some point, the recipient receives the token transaction that in the meantime has been assembled
	tx, err := pledge.ReceiveTransaction(context)
	assert.NoError(err, "failed to receive tokens")

	outputs, err := tx.Outputs()
	assert.NoError(err, "failed getting outputs")
	assert.True(outputs.Count() >= 1, "expected at least one output, got [%d]", outputs.Count())
	outputs = outputs.ByScript()
	assert.True(outputs.Count() == 1, "expected only one pledge output, got [%d]", outputs.Count())
	script := outputs.ScriptAt(0)
	assert.NotNil(script, "expected a pledge script")
	assert.Equal(me, script.Issuer, "Expected pledge script to have me (%x) as an issuer but it has %x instead", me, script.Issuer)

	// If everything is fine, the recipient accepts and sends back her signature.
	// Notice that, a signature from the recipient might or might not be required to make the transaction valid.
	// This depends on the driver implementation.
	_, err = context.RunView(pledge.NewAcceptView(tx))
	assert.NoError(err, "failed to accept new tokens")

	// The issue is in the same Fabric network of the pledger
	// Before completing, the recipient waits for finality of the transaction
	_, err = context.RunView(pledge.NewFinalityView(tx))
	assert.NoError(err, "new tokens were not committed")

	return nil, nil
}

// generatePledgeID generates a pledgeID randomly
func generatePledgeID() (string, error) {
	nonce, err := getRandomNonce()
	if err != nil {
		return "", errors.New("failed generating random nonce for pledgeID")
	}
	return hex.EncodeToString(nonce), nil
}

// getRandomNonce generates a random nonce using the package math/rand
func getRandomNonce() ([]byte, error) {
	key := make([]byte, 24)
	_, err := rand.Read(key)
	if err != nil {
		return nil, errors.Wrap(err, "error getting random bytes")
	}
	return key, nil
}
