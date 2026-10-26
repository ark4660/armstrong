package com.ark4660.armstrong.cosmetics;

import java.security.KeyFactory;
import java.security.PublicKey;
import java.security.Signature;
import java.security.spec.X509EncodedKeySpec;
import java.util.Base64;

/**
 * Checks cosmetics.json against the cosmetics signing key, so nobody can
 * hand out cosmetics by tampering with the download. The key is separate
 * from the release key (it can't sign updates); `armstrong cosmetics keygen`
 * printed it.
 */
public final class CosmeticsKey {
	static final String PUBLIC_KEY = "GFoRVgVHvrIN6RW/HeX0M98ZN41pBf0xd3mSYO+TR/w=";

	// X.509 prefix for a raw 32-byte Ed25519 key.
	private static final byte[] X509_PREFIX = {0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x03, 0x21, 0x00};

	private CosmeticsKey() {}

	/** Whether sigFile (base64, as `armstrong cosmetics` writes it) signs data. */
	static boolean verify(byte[] data, String sigFile) {
		try {
			byte[] raw = Base64.getDecoder().decode(PUBLIC_KEY);
			byte[] x509 = new byte[X509_PREFIX.length + raw.length];
			System.arraycopy(X509_PREFIX, 0, x509, 0, X509_PREFIX.length);
			System.arraycopy(raw, 0, x509, X509_PREFIX.length, raw.length);
			PublicKey key = KeyFactory.getInstance("Ed25519").generatePublic(new X509EncodedKeySpec(x509));
			Signature s = Signature.getInstance("Ed25519");
			s.initVerify(key);
			s.update(data);
			return s.verify(Base64.getDecoder().decode(sigFile.trim()));
		} catch (Exception e) {
			return false;
		}
	}
}
