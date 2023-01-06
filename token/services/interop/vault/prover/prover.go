/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package prover

import (
	"encoding/json"
	"time"

	"github.com/hyperledger-labs/fabric-smart-client/platform/view/services/hash"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/vault/keys"
	"github.com/hyperledger-labs/fabric-token-sdk/token/services/vault/translator"
	"github.com/hyperledger-labs/fabric-token-sdk/token/token"
	"github.com/pkg/errors"
)

const (
	ProofOfExistencePrefix         = "pe"
	ProofOfNonExistencePrefix      = "pne"
	ProofOfMetadataExistencePrefix = "pme"
)

type Metadata struct {
	// OriginTokenID is the identifier of the pledged token in the origin network
	OriginTokenID *token.ID
	// OriginNetwork is the network where the pledge took place
	OriginNetwork string
}

type Prover struct {
	namespace string
	RWSet     translator.RWSet
}

type ProofOfTokenMetadataNonExistence struct {
	Origin   string
	TokenID  *token.ID
	Deadline time.Time
}

type ProofOfTokenMetadataExistence struct {
	Origin  string
	TokenID *token.ID
}

// ProveTokenExists queries whether a token with the given token ID exists
func (p *Prover) ProveTokenExists(tokenId *token.ID) error {
	key, err := keys.CreateTokenKey(tokenId.TxId, tokenId.Index)
	if err != nil {
		return err
	}
	token, err := p.RWSet.GetState(p.namespace, key)
	if err != nil {
		return err
	}
	if token == nil {
		return errors.Errorf("value at key [%s] is empty", tokenId)
	}
	key, err = CreateProofOfExistenceKey(tokenId)
	if err != nil {
		return err
	}
	err = p.RWSet.SetState(p.namespace, key, token)
	if err != nil {
		return err
	}
	return nil
}

// ProveTokenDoesNotExist queries whether a token with metadata including the given token ID and origin network does not exist
func (p *Prover) ProveTokenDoesNotExist(tokenID *token.ID, origin string, deadline time.Time) error {
	if time.Now().Before(deadline) {
		return errors.Errorf("deadline has not elapsed yet")
	}
	metadata, err := json.Marshal(&Metadata{OriginTokenID: tokenID, OriginNetwork: origin})
	if err != nil {
		return errors.Errorf("failed to marshal token metadata")
	}
	key, err := keys.CreateIssueActionMetadataKey(hash.Hashable(metadata).String())
	if err != nil {
		return err
	}
	token, err := p.RWSet.GetState(p.namespace, key)
	if err != nil {
		return err
	}
	if token != nil {
		return errors.Errorf("value at key [%s] is not empty", key)
	}
	proof := &ProofOfTokenMetadataNonExistence{Origin: origin, TokenID: tokenID, Deadline: deadline}
	raw, err := json.Marshal(proof)
	if err != nil {
		return err
	}
	key, err = CreateProofOfNonExistenceKey(tokenID, origin)
	if err != nil {
		return err
	}
	err = p.RWSet.SetState(p.namespace, key, raw)
	if err != nil {
		return err
	}
	return nil
}

// ProveTokenWithMetadataExists queries whether a token with metadata including the given token ID and origin network exists
func (p *Prover) ProveTokenWithMetadataExists(tokenID *token.ID, origin string) error {
	metadata, err := json.Marshal(&Metadata{OriginTokenID: tokenID, OriginNetwork: origin})
	if err != nil {
		return errors.Errorf("failed to marshal token metadata")
	}
	key, err := keys.CreateIssueActionMetadataKey(hash.Hashable(metadata).String())
	if err != nil {
		return err
	}
	token, err := p.RWSet.GetState(p.namespace, key)
	if err != nil {
		return err
	}
	if token == nil {
		return errors.Errorf("value at key [%s] is empty", key)
	}
	proof := &ProofOfTokenMetadataExistence{Origin: origin, TokenID: tokenID}
	raw, err := json.Marshal(proof)
	if err != nil {
		return err
	}
	key, err = CreateProofOfMetadataExistenceKey(tokenID, origin)
	if err != nil {
		return err
	}
	err = p.RWSet.SetState(p.namespace, key, raw)
	if err != nil {
		return err
	}
	return nil
}

func New(rwSet translator.RWSet, namespace string) *Prover {
	return &Prover{RWSet: rwSet, namespace: namespace}
}

func CreateProofOfExistenceKey(tokenId *token.ID) (string, error) {
	id := hash.Hashable(tokenId.String()).String()
	return keys.CreateCompositeKey(ProofOfExistencePrefix, []string{id})
}

func CreateProofOfNonExistenceKey(tokenID *token.ID, origin string) (string, error) {
	return keys.CreateCompositeKey(ProofOfNonExistencePrefix, []string{
		hash.Hashable(tokenID.String()).String(),
		hash.Hashable(origin).String(),
	})
}

func CreateProofOfMetadataExistenceKey(tokenID *token.ID, origin string) (string, error) {
	return keys.CreateCompositeKey(ProofOfMetadataExistencePrefix, []string{
		hash.Hashable(tokenID.String()).String(),
		hash.Hashable(origin).String(),
	})
}
