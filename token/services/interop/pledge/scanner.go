/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package pledge

import (
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/network"
	"github.com/pkg/errors"
)

const (
	ScanForPledgeIDStartingTransaction = "pledge.PledgeIDExists.StartingTransaction"
)

// WithStartingTransaction sets the starting transaction for the scan
func WithStartingTransaction(txID string) token.ServiceOption {
	return func(o *token.ServiceOptions) error {
		if o.Params == nil {
			o.Params = map[string]interface{}{}
		}
		o.Params[ScanForPledgeIDStartingTransaction] = txID
		return nil
	}
}

// PledgeIDExists scans the ledger for a pledge identifier, taking into account the timeout
// PledgeIDExists returns true, if entry identified by key (PledgeKey+pledgeID) is occupied.
func PledgeIDExists(ctx view.Context, pledgeID string, timeout time.Duration, opts ...token.ServiceOption) (bool, error) {
	logger.Infof("scanning for pledgeID of [%s] with timeout [%s]", pledgeID, timeout)
	tokenOptions, err := token.CompileServiceOptions(opts...)
	if err != nil {
		return false, err
	}
	tms := token.GetManagementService(ctx, opts...)

	network := network.GetInstance(ctx, tms.Network(), tms.Channel())
	if network == nil {
		return false, errors.Errorf("cannot find network [%s:%s]", tms.Namespace(), tms.Channel())
	}

	startingTxID, err := tokenOptions.ParamAsString(ScanForPledgeIDStartingTransaction)
	if err != nil {
		return false, errors.Wrapf(err, "invalid starting transaction param")
	}

	pledgeKey := PledgeKey + pledgeID
	v, err := network.LookupTransferMetadataKey(tms.Namespace(), startingTxID, pledgeKey, timeout, opts...)
	if err != nil {
		return false, errors.Wrapf(err, "failed to lookup transfer metadata for pledge ID [%s]", pledgeID)
	}
	if len(v) != 0 {
		return true, nil
	}
	return false, nil
}
