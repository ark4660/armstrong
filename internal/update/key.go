package update

// PublicKey verifies release.json.sig (a var only so tests can use their own key). The matching private key signs
// releases; it lives outside the repository (and as the
// ARMSTRONG_SIGNING_KEY secret in GitHub Actions), never in the code.
var PublicKey = "qETw4i8CEEyQLS9PXcUh54/LIjWfCndjlptz/1z0heI="
