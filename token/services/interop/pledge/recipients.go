/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pledge

import (
	"encoding/json"
	"time"

	view2 "github.com/hyperledger-labs/fabric-smart-client/platform/view"
	session2 "github.com/hyperledger-labs/fabric-smart-client/platform/view/services/session"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/ttx"
	"github.com/pkg/errors"
)

func compileServiceOptions(opts ...token.ServiceOption) (*token.TMSID, error) {
	txOptions := &token.ServiceOptions{}
	for _, opt := range opts {
		if err := opt(txOptions); err != nil {
			return nil, err
		}
	}
	id := txOptions.TMSID()
	return &id, nil
}

type RecipientData struct {
	Identity  view.Identity
	AuditInfo []byte
	Metadata  []byte
}

func (r *RecipientData) Bytes() ([]byte, error) {
	return json.Marshal(r)
}

func (r *RecipientData) FromBytes(raw []byte) error {
	return json.Unmarshal(raw, r)
}

type RecipientRequest struct {
	Network  string
	WalletID []byte
}

func (r *RecipientRequest) Bytes() ([]byte, error) {
	return json.Marshal(r)
}

func (r *RecipientRequest) FromBytes(raw []byte) error {
	return json.Unmarshal(raw, r)
}

type RequestRecipientIdentityView struct {
	TMSID       token.TMSID
	DestNetwork string
	Other       view.Identity
}

// RequestPledgeRecipientIdentity executes the RequestRecipientIdentityView.
// The sender contacts the recipient's FSC node identified via the passed view identity.
// The sender gets back the identity the recipient wants to use to assign ownership of tokens.
func RequestPledgeRecipientIdentity(context view.Context, recipient view.Identity, destNetwork string, opts ...token.ServiceOption) (view.Identity, error) {
	tmsID, err := compileServiceOptions(opts...)
	if err != nil {
		return nil, err
	}
	pseudonymBoxed, err := context.RunView(&RequestRecipientIdentityView{
		TMSID:       *tmsID,
		DestNetwork: destNetwork,
		Other:       recipient,
	})
	if err != nil {
		return nil, err
	}
	return pseudonymBoxed.(view.Identity), nil
}

func (f RequestRecipientIdentityView) Call(context view.Context) (interface{}, error) {
	logger.Debugf("request recipient to [%s] for TMS [%s]", f.Other, f.TMSID)

	tms := token.GetManagementService(context, token.WithTMSID(f.TMSID))

	if w := tms.WalletManager().OwnerWalletByIdentity(f.Other); w != nil {
		recipient, err := w.GetRecipientIdentity()
		if err != nil {
			return nil, err
		}
		return recipient, nil
	} else {
		session, err := context.GetSession(context.Initiator(), f.Other)
		if err != nil {
			return nil, err
		}

		// Ask for identity
		rr := &RecipientRequest{
			Network:  f.DestNetwork,
			WalletID: f.Other,
		}
		rrRaw, err := rr.Bytes()
		if err != nil {
			return nil, errors.Wrapf(err, "failed marshalling recipient request")
		}
		err = session.Send(rrRaw)
		if err != nil {
			return nil, err
		}

		// Wait to receive a view identity
		ch := session.Receive()
		var payload []byte
		select {
		case msg := <-ch:
			payload = msg.Payload
		case <-time.After(60 * time.Second):
			return nil, errors.New("time out reached")
		}

		recipientData := &RecipientData{}
		if err := recipientData.FromBytes(payload); err != nil {
			return nil, errors.Wrapf(err, "failed unmarshall payload [%s]", string(payload))
		}
		if err := tms.WalletManager().RegisterRecipientIdentity(recipientData.Identity, recipientData.AuditInfo, recipientData.Metadata); err != nil {
			return nil, err
		}

		// Update the Endpoint Resolver
		if err := view2.GetEndpointService(context).Bind(f.Other, recipientData.Identity); err != nil {
			return nil, err
		}

		return recipientData.Identity, nil
	}
}

type RespondRequestPledgeRecipientIdentityView struct {
	Wallet string
}

func (s *RespondRequestPledgeRecipientIdentityView) Call(context view.Context) (interface{}, error) {
	session, payload, err := session2.ReadFirstMessage(context)
	if err != nil {
		return nil, err
	}

	recipientRequest := &RecipientRequest{}
	if err := recipientRequest.FromBytes(payload); err != nil {
		return nil, errors.Wrapf(err, "failed unmarshalling recipient request")
	}

	wallet := s.Wallet
	if len(wallet) == 0 && len(recipientRequest.WalletID) != 0 {
		wallet = string(recipientRequest.WalletID)
	}

	tmsID, err := FabricURLToTMSID(recipientRequest.Network)
	if err != nil {
		return nil, errors.Wrapf(err, "failed parsing destination [%s]", recipientRequest.Network)
	}
	w := GetWallet(
		context,
		wallet,
		token.WithTMSID(tmsID),
	)
	if w == nil {
		return nil, errors.Errorf("unable to get wallet %s in %s", wallet, tmsID)
	}
	recipientIdentity, err := w.GetRecipientIdentity()
	if err != nil {
		return nil, err
	}
	auditInfo, err := w.GetAuditInfo(recipientIdentity)
	if err != nil {
		return nil, err
	}
	metadata, err := w.GetTokenMetadata(recipientIdentity)
	if err != nil {
		return nil, err
	}
	recipientData := &RecipientData{
		Identity:  recipientIdentity,
		AuditInfo: auditInfo,
		Metadata:  metadata,
	}
	recipientDataRaw, err := recipientData.Bytes()
	if err != nil {
		return nil, err
	}

	// Step 3: send the public key back to the invoker
	err = session.Send(recipientDataRaw)
	if err != nil {
		return nil, err
	}

	// Update the Endpoint Resolver
	resolver := view2.GetEndpointService(context)
	err = resolver.Bind(context.Me(), recipientIdentity)
	if err != nil {
		return nil, err
	}

	return recipientIdentity, nil
}

// RespondRequestPledgeRecipientIdentity executes the RespondRequestPledgeRecipientIdentityView.
// The recipient sends back the identity to receive ownership of tokens.
// The identity is taken from the wallet
func RespondRequestPledgeRecipientIdentity(context view.Context) (view.Identity, error) {
	id, err := context.RunView(&RespondRequestPledgeRecipientIdentityView{})
	if err != nil {
		return nil, err
	}
	return id.(view.Identity), nil
}

// RequestRecipientIdentity executes the RequestRecipientIdentityView.
func RequestRecipientIdentity(context view.Context, recipient view.Identity, opts ...token.ServiceOption) (view.Identity, error) {
	return ttx.RequestRecipientIdentity(context, recipient, opts...)
}

// RespondRequestRecipientIdentity executes the RespondRequestRecipientIdentityView.
func RespondRequestRecipientIdentity(context view.Context) (view.Identity, error) {
	return ttx.RespondRequestRecipientIdentity(context)
}
