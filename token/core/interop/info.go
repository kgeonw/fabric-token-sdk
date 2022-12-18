/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package interop

import (
	"encoding/json"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/identity"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/interop/htlc"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/interop/pledge"
	"github.com/pkg/errors"
)

type AuditInfoProvider interface {
	GetAuditInfo(identity view.Identity) ([]byte, error)
}

// GetOwnerAuditInfo returns the audit info of the owner
func GetOwnerAuditInfo(raw []byte, s AuditInfoProvider) ([]byte, error) {
	if len(raw) == 0 {
		// this is a redeem
		return nil, nil
	}

	owner, err := identity.UnmarshallRawOwner(raw)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal owner of input token")
	}
	if owner.Type == identity.SerializedIdentityType {
		auditInfo, err := s.GetAuditInfo(raw)
		if err != nil {
			return nil, errors.Wrapf(err, "failed getting audit info for recipient identity [%s]", view.Identity(raw).String())
		}
		return auditInfo, nil
	}

	sender, recipient, _, err := GetScriptSenderAndRecipient(owner)
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting script sender and recipient")
	}

	auditInfo := &ScriptInfo{}
	auditInfo.Sender, err = s.GetAuditInfo(sender)
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting audit info for htlc script [%s]", view.Identity(raw).String())
	}

	auditInfo.Recipient, err = s.GetAuditInfo(recipient)
	if err != nil {
		return nil, errors.Wrapf(err, "failed getting audit info for script [%s]", view.Identity(raw).String())
	}
	raw, err = json.Marshal(auditInfo)
	if err != nil {
		return nil, errors.Wrapf(err, "failed marshaling audit info for script")
	}
	return raw, nil
}

// ScriptInfo includes info about the sender and the recipient
type ScriptInfo struct {
	Sender    []byte
	Recipient []byte
}

// GetScriptSenderAndRecipient returns the script's sender, recipient, and issuer, according to the type of the given owner
func GetScriptSenderAndRecipient(ro *identity.RawOwner) (sender, recipient, issuer view.Identity, err error) {
	if ro.Type == htlc.ScriptType {
		script := &htlc.Script{}
		err = json.Unmarshal(ro.Identity, script)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "failed to unmarshal htlc script")
		}
		return script.Sender, script.Recipient, nil, nil
	}
	if ro.Type == pledge.ScriptType {
		script := &pledge.Script{}
		err = json.Unmarshal(ro.Identity, script)
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "failed to unmarshal pledge script")
		}
		return script.Sender, script.Recipient, script.Issuer, nil
	}
	return nil, nil, nil, errors.Errorf("owner's type not recognized [%s]", ro.Type)
}
