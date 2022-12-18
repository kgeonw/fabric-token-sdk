/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pledge

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/identity"
)

const (
	PledgeKey             = "metadata.pledge"
	defaultDeadlineOffset = time.Hour
)

func (t *Transaction) Pledge(wallet *token.OwnerWallet, destNetwork string, deadline time.Duration, recipient view.Identity, issuer view.Identity, pledgeID string, typ string, value uint64) error {
	if deadline == 0 {
		deadline = defaultDeadlineOffset
	}
	if destNetwork == "" {
		return fmt.Errorf("must specify a destination network")
	}
	if issuer.IsNone() {
		return fmt.Errorf("must specify an issuer")
	}
	if recipient.IsNone() {
		return fmt.Errorf("must specify a recipient")
	}
	me, err := wallet.GetRecipientIdentity()
	if err != nil {
		return err
	}
	script, err := t.recipientAsScript(me, destNetwork, deadline, recipient, issuer, pledgeID)
	if err != nil {
		return err
	}
	_, err = t.TokenRequest.Transfer(wallet, typ, []uint64{value}, []view.Identity{script}, token.WithTransferMetadata(PledgeKey+pledgeID, []byte("1")))
	return err
}

func (t *Transaction) recipientAsScript(sender view.Identity, destNetwork string, deadline time.Duration, recipient view.Identity, issuer view.Identity, pledgeID string) (view.Identity, error) {
	script := Script{
		Deadline:           time.Now().Add(deadline),
		DestinationNetwork: destNetwork,
		Recipient:          recipient,
		Issuer:             issuer,
		Sender:             sender,
		ID:                 pledgeID,
	}
	rawScript, err := json.Marshal(script)
	if err != nil {
		return nil, err
	}

	ro := &identity.RawOwner{
		Type:     ScriptType,
		Identity: rawScript,
	}
	return identity.MarshallRawOwner(ro)
}
