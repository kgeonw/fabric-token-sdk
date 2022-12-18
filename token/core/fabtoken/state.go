/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package fabtoken

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/platform/fabric"
	weaver2 "github.com/hyperledger-labs/fabric-smart-client/platform/fabric/services/weaver"
	"github.com/hyperledger-labs/fabric-smart-client/platform/view"
	"github.com/hyperledger-labs/fabric-token-sdk/token/core/identity"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/interop/pledge"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/interop/vault/prover"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/network/fabric/tcc"
	"github.com/hyperledger-labs/fabric-token-sdk/token/token"
	"github.com/pkg/errors"
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

type StateVerifier struct {
	NetworkURL    string
	SP            view.ServiceProvider
	RelaySelector *fabric.NetworkService
}

func (v *StateVerifier) VerifyProofExistence(proofRaw []byte, tokenID *token.ID, metadata []byte) error {
	relay := weaver2.GetProvider(v.SP).Relay(v.RelaySelector)
	proof, err := relay.ToFabric().ProofFromBytes(proofRaw)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal claim proof")
	}
	if err := proof.Verify(); err != nil {
		return errors.Wrapf(err, "failed to verify pledge proof")
	}

	rwset, err := proof.RWSet()
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal claim proof")
	}

	key, err := prover.CreateProofOfExistenceKey(tokenID)
	if err != nil {
		return err
	}
	tmsID, err := pledge.FabricURLToTMSID(v.NetworkURL)
	if err != nil {
		return err
	}
	raw, err := rwset.GetState(tmsID.Namespace, key)
	if err != nil {
		return errors.Wrapf(err, "failed to check proof of existence")
	}
	tok := &token.Token{}
	err = json.Unmarshal(raw, tok)
	if err != nil {
		return err
	}
	// Validate against pledge
	logger.Debugf("verify proof of existence for token id [%s]", tokenID)
	pledges, err := pledge.PledgeVault(v.SP).PledgeByTokenID(tokenID)
	if err != nil {
		logger.Errorf("failed retrieving pledge info for token id [%s]: [%s]", tokenID, err)
		return errors.WithMessagef(err, "failed getting pledge for [%s]", tokenID)
	}
	if len(pledges) != 1 {
		logger.Errorf("failed retrieving pledge info for token id [%s]: no info found", tokenID)
		return errors.Errorf("expected one pledge, got [%d]", len(pledges))
	}
	info := pledges[0]
	logger.Debugf("found pledge info for token id [%s]: [%s]", tokenID, info.Source)

	if tok.Type != info.TokenType {
		return errors.Errorf("type of pledge token does not match type in claim request")
	}
	q, err := token.ToQuantity(tok.Quantity, 64)
	if err != nil {
		return err
	}
	expectedQ := token.NewQuantityFromUInt64(info.Amount)
	if expectedQ.Cmp(q) != 0 {
		return errors.Errorf("quantity in pledged token is different from quantity in claim request")
	}
	owner, err := identity.UnmarshallRawOwner(tok.Owner.Raw)
	if err != nil {
		return err
	}
	if owner.Type != pledge.ScriptType {
		return err
	}
	script := &pledge.Script{}
	err = json.Unmarshal(owner.Identity, script)
	if err != nil {
		return err
	}
	if script.Recipient == nil {
		return errors.Errorf("script in proof encodes invalid recipient")
	}
	if !script.Recipient.Equal(info.Script.Recipient) {
		return errors.Errorf("recipient in claim request does not match recipient in proof")
	}
	if script.Deadline != info.Script.Deadline {
		return errors.Errorf("deadline in claim request does not match deadline in proof")
	}
	if script.DestinationNetwork != info.Script.DestinationNetwork {
		fmt.Printf("[%s vs.%s] \n", script.DestinationNetwork, info.Script.DestinationNetwork)
		return errors.Errorf("destination network in claim request does not match destination network in proof")
	}

	return nil
}

func (v *StateVerifier) VerifyProofNonExistence(proofRaw []byte, tokenID *token.ID, origin string, deadline time.Time) error {
	// v.NetworkURL is the network from which the proof comes from
	tokenOriginNetworkTMSID, err := pledge.FabricURLToTMSID(origin)
	if err != nil {
		return errors.Wrapf(err, "failed to parse network url")
	}
	relay := weaver2.GetProvider(v.SP).Relay(fabric.GetFabricNetworkService(v.SP, tokenOriginNetworkTMSID.Network))
	proof, err := relay.ToFabric().ProofFromBytes(proofRaw)
	if err != nil {
		return errors.Wrapf(err, "failed to umarshal proof")
	}

	rwset, err := proof.RWSet()
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve RWset")
	}

	key, err := prover.CreateProofOfNonExistenceKey(tokenID, origin)
	if err != nil {
		return errors.Wrapf(err, "failed creating key for proof of non-existence")
	}

	proofSourceNetworkTMSID, err := pledge.FabricURLToTMSID(v.NetworkURL)
	if err != nil {
		return err
	}
	raw, err := rwset.GetState(proofSourceNetworkTMSID.Namespace, key)
	if err != nil {
		return errors.Wrapf(err, "failed to check proof of non-existence")
	}
	p := &prover.ProofOfTokenMetadataNonExistence{}
	if raw == nil {
		return errors.Errorf("could not find proof of non-existence")
	}
	err = json.Unmarshal(raw, p)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal proof of non-existence")
	}
	if p.Deadline != deadline {
		return errors.Errorf("deadline in reclaim request does not match deadline in proof of non-existence")
	}
	if p.TokenID.String() != tokenID.String() {
		return errors.Errorf("token ID in reclaim request does not match token ID in proof of non-existence")
	}
	if p.Origin != pledge.FabricURL(tokenOriginNetworkTMSID) {
		return errors.Errorf("origin in reclaim request does not match origin in proof of non-existence")
	}

	// todo check that address in proof is the destination network

	err = proof.Verify()
	if err != nil {
		return errors.Wrapf(err, "invalid proof of non-existence")
	}

	return nil
}

// VerifyProofTokenWithMetadataExistence verifies that a proof of existence of a token
// with metadata including the given token ID and origin network, in the target network is valid
func (v *StateVerifier) VerifyProofTokenWithMetadataExistence(proofRaw []byte, tokenID *token.ID, origin string) error {
	// v.NetworkURL is the network from which the proof comes from
	tokenOriginNetworkTMSID, err := pledge.FabricURLToTMSID(origin)
	if err != nil {
		return errors.Wrapf(err, "failed to parse network url")
	}
	relay := weaver2.GetProvider(v.SP).Relay(fabric.GetFabricNetworkService(v.SP, tokenOriginNetworkTMSID.Network))
	proof, err := relay.ToFabric().ProofFromBytes(proofRaw)
	if err != nil {
		return errors.Wrapf(err, "failed to umarshal proof")
	}

	rwset, err := proof.RWSet()
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve RWset")
	}

	key, err := prover.CreateProofOfMetadataExistenceKey(tokenID, origin)
	if err != nil {
		return errors.Wrapf(err, "failed creating key for proof of token existence")
	}

	proofSourceNetworkTMSID, err := pledge.FabricURLToTMSID(v.NetworkURL)
	if err != nil {
		return err
	}
	raw, err := rwset.GetState(proofSourceNetworkTMSID.Namespace, key)
	if err != nil {
		return errors.Wrapf(err, "failed to check proof of token existence")
	}
	p := &prover.ProofOfTokenMetadataExistence{}
	if raw == nil {
		return errors.Errorf("could not find proof of token existence")
	}
	err = json.Unmarshal(raw, p)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal proof of token existence")
	}
	if p.TokenID.String() != tokenID.String() {
		return errors.Errorf("token ID in redeem request does not match token ID in proof of token existence")
	}
	if p.Origin != pledge.FabricURL(tokenOriginNetworkTMSID) {
		return errors.Errorf("origin in redeem request does not match origin in proof of token existence")
	}

	// todo check that address in proof is the destination network

	err = proof.Verify()
	if err != nil {
		return errors.Wrapf(err, "invalid proof of token existence")
	}

	return nil
}
