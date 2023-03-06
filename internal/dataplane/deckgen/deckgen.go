package deckgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"hash"
	"hash/crc32"
	"sync"

	gojson "github.com/goccy/go-json"
	"github.com/kong/deck/file"
	"github.com/kong/go-kong/kong"
	"github.com/mitchellh/hashstructure/v2"
)

// GenerateSHA generates a SHA256 checksum of targetContent, with the purpose
// of change detection.
func GenerateSHA(targetContent *file.Content) ([]byte, error) {
	jsonConfig, err := json.Marshal(targetContent)
	if err != nil {
		return nil, fmt.Errorf("marshaling Kong declarative configuration to JSON: %w", err)
	}

	shaSum := sha256.Sum256(jsonConfig)
	return shaSum[:], nil
}

var _hashPool = sync.Pool{
	New: func() any {
		return sha256.New()
	},
}

var _targetConfigCache = make(map[uint64][]byte)

func GenerateSHA_gojson_cache(targetContent *file.Content) ([]byte, error) {
	hash, err := hashstructure.Hash(targetContent, hashstructure.FormatV2, nil)
	if err != nil {
		panic(err)
	}
	if sha, ok := _targetConfigCache[hash]; ok {
		// fmt.Printf("found %v\n", hash)
		return sha, nil
	}

	sha, err := GenerateSHA_gojson(targetContent)
	if err != nil {
		return nil, err
	}

	_targetConfigCache[hash] = sha

	return sha, nil
}

var _crc32_encoderPool = sync.Pool{
	New: func() any {
		return crc32.New(crc32.MakeTable(crc32.Castagnoli))
	},
}

func GenerateSHA_gob_crc32(targetContent *file.Content) ([]byte, error) {
	encHasher := _crc32_encoderPool.Get().(hash.Hash32)
	encHasher.Reset()

	enc := gob.NewEncoder(encHasher)
	err := enc.Encode(targetContent)
	if err != nil {
		return nil, fmt.Errorf("encoding target content failed: %w", err)
	}

	hash := make([]byte, 4)
	binary.LittleEndian.PutUint32(hash, encHasher.Sum32())
	_crc32_encoderPool.Put(encHasher)

	return hash, nil
}

var _bytesPool = sync.Pool{
	New: func() any {
		return bytes.Buffer{}
	},
}

func GenerateSHA_gob_sha256(targetContent *file.Content) ([]byte, error) {
	b := _bytesPool.Get().(bytes.Buffer)
	b.Reset()
	enc := gob.NewEncoder(&b)
	err := enc.Encode(targetContent)
	if err != nil {
		return nil, fmt.Errorf("encoding target content failed: %w", err)
	}

	shaSum := sha256.Sum256(b.Bytes())
	_bytesPool.Put(b)
	return shaSum[:], nil
}

func GenerateSHA_gojson(targetContent *file.Content) ([]byte, error) {
	jsonConfig, err := gojson.Marshal(targetContent)
	if err != nil {
		return nil, fmt.Errorf("marshaling Kong declarative configuration to JSON: %w", err)
	}

	shaSum := sha256.New()
	_, err = shaSum.Write(jsonConfig)
	if err != nil {
		return nil, fmt.Errorf("marshaling Kong declarative configuration to JSON: %w", err)
	}
	sha := shaSum.Sum(nil)

	return sha, nil
}

var _bytesGojsonPool = sync.Pool{
	New: func() any {
		return bytes.Buffer{}
	},
}

func GenerateSHA_gojson_nopool(targetContent *file.Content) ([]byte, error) {
	b := bytes.Buffer{}
	enc := gojson.NewEncoder(&b)

	if err := enc.Encode(targetContent); err != nil {
		return nil, fmt.Errorf("marshaling Kong declarative configuration to JSON: %w", err)
	}

	shaSum := sha256.New()
	shaSum.Reset()
	if _, err := shaSum.Write(b.Bytes()); err != nil {
		return nil, fmt.Errorf("marshaling Kong declarative configuration to JSON: %w", err)
	}

	return shaSum.Sum(nil), nil
}

func GenerateSHA_gojson_pool(targetContent *file.Content) ([]byte, error) {
	b := _bytesGojsonPool.Get().(bytes.Buffer)
	b.Reset()
	enc := gojson.NewEncoder(&b)

	if err := enc.Encode(targetContent); err != nil {
		return nil, fmt.Errorf("marshaling Kong declarative configuration to JSON: %w", err)
	}

	shaSum := _hashPool.Get().(hash.Hash)
	shaSum.Reset()
	if _, err := shaSum.Write(b.Bytes()); err != nil {
		return nil, fmt.Errorf("marshaling Kong declarative configuration to JSON: %w", err)
	}
	sha := shaSum.Sum(nil)
	shaSum.Reset()
	_hashPool.Put(shaSum)
	_bytesGojsonPool.Put(b)

	return sha, nil
}

// CleanUpNullsInPluginConfigs modifies `state` by deleting plugin config map keys that have nil as their value.
func CleanUpNullsInPluginConfigs(state *file.Content) {
	for _, s := range state.Services {
		for _, p := range s.Plugins {
			for k, v := range p.Config {
				if v == nil {
					delete(p.Config, k)
				}
			}
		}
		for _, r := range state.Routes {
			for _, p := range r.Plugins {
				for k, v := range p.Config {
					if v == nil {
						delete(p.Config, k)
					}
				}
			}
		}
	}

	for _, c := range state.Consumers {
		for _, p := range c.Plugins {
			for k, v := range p.Config {
				if v == nil {
					delete(p.Config, k)
				}
			}
		}
	}

	for _, p := range state.Plugins {
		for k, v := range p.Config {
			if v == nil {
				delete(p.Config, k)
			}
		}
	}
}

// GetFCertificateFromKongCert converts a kong.Certificate to a file.FCertificate.
func GetFCertificateFromKongCert(kongCert kong.Certificate) file.FCertificate {
	var res file.FCertificate
	if kongCert.ID != nil {
		res.ID = kong.String(*kongCert.ID)
	}
	if kongCert.Key != nil {
		res.Key = kong.String(*kongCert.Key)
	}
	if kongCert.Cert != nil {
		res.Cert = kong.String(*kongCert.Cert)
	}
	res.SNIs = getSNIs(kongCert.SNIs)
	return res
}

func getSNIs(names []*string) []kong.SNI {
	var snis []kong.SNI
	for _, name := range names {
		snis = append(snis, kong.SNI{
			Name: kong.String(*name),
		})
	}
	return snis
}

// PluginString returns a string representation of a FPlugin suitable as a sorting key.
//
// Deprecated. To be replaced by a predicate that compares two FPlugins.
func PluginString(plugin file.FPlugin) string {
	result := ""
	if plugin.Name != nil {
		result = *plugin.Name
	}
	if plugin.Consumer != nil && plugin.Consumer.ID != nil {
		result += *plugin.Consumer.ID
	}
	if plugin.Route != nil && plugin.Route.ID != nil {
		result += *plugin.Route.ID
	}
	if plugin.Service != nil && plugin.Service.ID != nil {
		result += *plugin.Service.ID
	}
	return result
}
