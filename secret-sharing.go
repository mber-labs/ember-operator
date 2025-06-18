package main

import (
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/btcsuite/btcd/btcec"
)

// var curveOrder, _ = new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)

func EvaluatePolynomial(coefficients []*big.Int, x *big.Int) *big.Int {
	result := big.NewInt(0)
	for i, coeff := range coefficients {
		term := new(big.Int).Exp(x, big.NewInt(int64(i)), elliptic.P256().Params().N) // x^i
		term.Mul(term, coeff)                                                         // coeff * x^i
		result.Add(result, term)
	}
	return result.Mod(result, elliptic.P256().Params().N)
}

func GenerateCoefficients(secret *big.Int, threshold int) []*big.Int {
	coefficients := make([]*big.Int, threshold)
	coefficients[0] = secret // First coefficient is the secret
	for i := 1; i < threshold; i++ {
		randomCoeff, _ := rand.Int(rand.Reader, elliptic.P256().Params().N)
		coefficients[i] = randomCoeff
	}
	return coefficients
}

func SplitSecret(secret *big.Int, totalShares, threshold int) [][2]*big.Int {
	coefficients := GenerateCoefficients(secret, threshold)
	shares := make([][2]*big.Int, totalShares)

	for i := 1; i <= totalShares; i++ {
		x := big.NewInt(int64(i))
		y := EvaluatePolynomial(coefficients, x)
		shares[i-1] = [2]*big.Int{x, y}
	}
	return shares
}

func PartialSign(share [2]*big.Int, message []byte, nonce *big.Int) *big.Int {
	hash := sha256.Sum256(message)

	// Partial signature: si = ki + f(i) * hash mod curveOrder
	partialSig := new(big.Int).Mul(share[1], new(big.Int).SetBytes(hash[:]))
	partialSig.Add(partialSig, nonce)
	return partialSig.Mod(partialSig, elliptic.P256().Params().N)
}

func Sign(share *big.Int, message []byte, nonce *big.Int) *big.Int {
	hash := sha256.Sum256(message)

	// Partial signature: si = ki + f(i) * hash mod curveOrder
	partialSig := new(big.Int).Mul(share, new(big.Int).SetBytes(hash[:]))
	partialSig.Add(partialSig, nonce)
	return partialSig.Mod(partialSig, elliptic.P256().Params().N)
}

func CombinePartialSigs(partialSigs map[int]*big.Int, shares [][2]*big.Int, message []byte) *big.Int {
	finalSig := big.NewInt(0)

	for i, partialSigI := range partialSigs {
		numerator := big.NewInt(1)
		denominator := big.NewInt(1)

		for j, shareJ := range shares {
			if i != j {
				numerator.Mul(numerator, shareJ[0])
				denominator.Mul(denominator, new(big.Int).Sub(shareJ[0], shares[i][0]))
			}
		}

		lagrangeCoeff := new(big.Int).Mul(numerator, new(big.Int).ModInverse(denominator, elliptic.P256().Params().N))
		lagrangeCoeff.Mod(lagrangeCoeff, elliptic.P256().Params().N)

		term := new(big.Int).Mul(partialSigI, lagrangeCoeff)
		finalSig.Add(finalSig, term)
	}
	return finalSig.Mod(finalSig, elliptic.P256().Params().N)
}

func ReconstructSecret(shares [][2]*big.Int) *big.Int {
	secret := big.NewInt(0)
	for i, shareI := range shares {
		numerator := big.NewInt(1)
		denominator := big.NewInt(1)

		for j, shareJ := range shares {
			if i != j {
				numerator.Mul(numerator, shareJ[0])
				denominator.Mul(denominator, new(big.Int).Sub(shareJ[0], shareI[0]))
			}
		}

		lagrangeCoeff := new(big.Int).Mul(numerator, new(big.Int).ModInverse(denominator, elliptic.P256().Params().N))
		lagrangeCoeff.Mod(lagrangeCoeff, elliptic.P256().Params().N)

		term := new(big.Int).Mul(shareI[1], lagrangeCoeff)
		secret.Add(secret, term)
	}
	return secret.Mod(secret, elliptic.P256().Params().N)
}

func ReconstructPubKey(shares [][2]*big.Int) *big.Int {
	secret := big.NewInt(0)
	for i, shareI := range shares {
		numerator := big.NewInt(1)
		denominator := big.NewInt(1)

		for j, shareJ := range shares {
			if i != j {
				numerator.Mul(numerator, shareJ[0])
				denominator.Mul(denominator, new(big.Int).Sub(shareJ[0], shareI[0]))
			}
		}

		lagrangeCoeff := new(big.Int).Mul(numerator, new(big.Int).ModInverse(denominator, elliptic.P256().Params().N))
		lagrangeCoeff.Mod(lagrangeCoeff, elliptic.P256().Params().N)

		term := new(big.Int).Mul(shareI[1], lagrangeCoeff)
		secret.Add(secret, term)
	}
	return secret.Mod(secret, elliptic.P256().Params().N)
}

func ReconstructPublicKey(shares []*btcec.PublicKey, indices []*big.Int) []byte {
	curve := btcec.S256()
	reconstructedX, reconstructedY := big.NewInt(0), big.NewInt(0)

	for i, share := range shares {
		num, denom := big.NewInt(1), big.NewInt(1)
		for j, idx := range indices {
			if i != j {
				num.Mul(num, idx).Mod(num, curve.Params().N)
				denom.Mul(denom, new(big.Int).Sub(idx, indices[i])).Mod(denom, curve.Params().N)
			}
		}

		lagrangeCoeff := new(big.Int).Mul(num, new(big.Int).ModInverse(denom, curve.Params().N))
		lagrangeCoeff.Mod(lagrangeCoeff, curve.Params().N)

		scaledX, scaledY := curve.ScalarMult(share.X, share.Y, lagrangeCoeff.Bytes())
		reconstructedX, reconstructedY = curve.Add(reconstructedX, reconstructedY, scaledX, scaledY)
	}

	pubKey := append([]byte{0x02 + byte(reconstructedY.Bit(0))}, reconstructedX.Bytes()...)
	return pubKey
}

func pvtKeyToString(share [2]*big.Int) string {
	return fmt.Sprintf("%x,%x", share[0], share[1])
}

func stringToPvtKey(s string) ([2]*big.Int, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return [2]*big.Int{}, errors.New("invalid share string format")
	}
	x := new(big.Int)
	y := new(big.Int)

	_, ok1 := x.SetString(parts[0], 16)
	_, ok2 := y.SetString(parts[1], 16)

	if !ok1 || !ok2 {
		return [2]*big.Int{}, errors.New("failed to parse big.Int values")
	}

	return [2]*big.Int{x, y}, nil
}
