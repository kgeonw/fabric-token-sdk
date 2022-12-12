/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fabric

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/platform/fabric"
	weaver2 "github.com/hyperledger-labs/fabric-smart-client/platform/fabric/services/weaver"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/network/fabric/tcc"
	"github.com/hyperledger-labs/fabric-token-sdk/token/token"
)

type StateQueryExecutor struct {
	TargetNetworkURL string
	SP               view.ServiceProvider
	RelaySelector    *fabric.NetworkService
}

func (p *StateQueryExecutor) Exist(tokenID *token.ID) ([]byte, error) {
	raw, err := json.Marshal(tokenID)
	if err != nil {
		return nil, err
	}

	relay := weaver2.GetProvider(p.SP).Relay(p.RelaySelector)
	logger.Debugf("Query [%s] for proof of existence of token [%s], input [%s]", p.TargetNetworkURL, tokenID.String(), base64.StdEncoding.EncodeToString(raw))

	query, err := relay.ToFabric().Query(p.TargetNetworkURL, tcc.ProofOfTokenExistenceQuery, base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		return nil, err
	}
	res, err := query.Call()
	if err != nil {
		return nil, err
	}

	return res.Proof()
}

func (p *StateQueryExecutor) DoesNotExist(tokenID *token.ID, origin string, deadline time.Time) ([]byte, error) {
	req := &tcc.ProofOfTokenNonExistenceRequest{
		Deadline:      deadline,
		OriginNetwork: origin,
		TokenID:       tokenID,
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	relay := weaver2.GetProvider(p.SP).Relay(p.RelaySelector)

	logger.Debugf("Query [%s] for proof of non-existence of token [%s], input [%s]", p.TargetNetworkURL, tokenID.String(), base64.StdEncoding.EncodeToString(raw))

	query, err := relay.ToFabric().Query(p.TargetNetworkURL, tcc.ProofOfTokenNonExistenceQuery, base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		return nil, err
	}
	res, err := query.Call()
	if err != nil {
		return nil, err
	}
	return res.Proof()
}

// ExistsWithMetadata returns a proof that a token with metadata including the passed token ID and origin network exists
// in the network this query executor targets
func (p *StateQueryExecutor) ExistsWithMetadata(tokenID *token.ID, origin string) ([]byte, error) {
	req := &tcc.ProofOfTokenMetadataExistenceRequest{
		OriginNetwork: origin,
		TokenID:       tokenID,
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	relay := weaver2.GetProvider(p.SP).Relay(p.RelaySelector)

	logger.Debugf("Query [%s] for proof of existence of metadata with token [%s], input [%s]", p.TargetNetworkURL, tokenID.String(), base64.StdEncoding.EncodeToString(raw))

	query, err := relay.ToFabric().Query(p.TargetNetworkURL, tcc.ProofOfTokenMetadataExistenceQuery, base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		return nil, err
	}
	res, err := query.Call()
	if err != nil {
		return nil, err
	}
	return res.Proof()
}
