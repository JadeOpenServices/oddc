# ODDC releases

A release is a signed git tag on `main`. A consumer that mirrors ODDC can
check, offline and without trusting GitHub, that a revision was released
by a maintainer.

## Tags

- Name: `vYYYY.MM.N`, the year and month of the release and a counter
  from 0 within that month, such as `v2026.10.0`, `v2026.10.1`.
- Annotated and signed (`git tag -s`), on a commit that is on `main`.
- Message: `ODDC vYYYY.MM.N`, then the ODDC items since the last release.
- A tag is never moved, deleted or reused. A bad release is followed by a
  new one.

Untagged commits on `main` are still what `github:JadeOpenServices/oddc`
follows. Tags are for consumers that only take released revisions.

## Who signs

Maintainers listed in [`keys/allowed_signers`](../keys/allowed_signers).
Each signing key is an SSH `ed25519-sk` key held on a physical security
key, which needs a touch and a PIN for every signature. The private key
never leaves the device. Each maintainer also enrolls a second security
key as a backup, kept apart from the first.

Today the only maintainer is Jade.

## The allowed signers file

`keys/allowed_signers` is in git's `gpg.ssh.allowedSignersFile` format, one
line per key:

    PRINCIPAL namespaces="git" sk-ssh-ed25519@openssh.com AAAA... COMMENT

- Only `sk-ssh-ed25519@openssh.com` keys are accepted; the tests refuse
  any other line.
- A key is retired by adding `valid-before="YYYYMMDD"` to its line,
  never by deleting it, so older tags keep verifying.
- A change to the file ships in a release signed by a key that the
  previous file already allows.

The copy in the repository records who may sign. It is **not** what a
consumer verifies against: whoever can change the tree could also change
that file. A consumer pins its own copy, checks the key fingerprints out
of band when it first pins it, and updates the pin only after reviewing a
change to `keys/allowed_signers` in a release it has already verified.

## Verifying a release

Needs git and OpenSSH 8.2 or later, without network or security key:

    git -c gpg.ssh.allowedSignersFile=/path/to/pinned/allowed_signers \
        verify-tag v2026.10.0
    git merge-base --is-ancestor v2026.10.0 main

`verify-tag` exits non-zero unless the tag is signed by a pinned key that
was valid when it signed. The second command checks the tag is on `main`.
Then `git rev-parse 'v2026.10.0^{commit}'` is the revision to use, and
`oddc validate --json` from that revision checks its catalog.

## Making a release (maintainer)

Once, per security key:

    ssh-keygen -t ed25519-sk -O resident -O verify-required \
        -O application=ssh:oddc-release \
        -C "ODDC release signing" -f ~/.ssh/oddc_release_sk
    printf '%s namespaces="git" %s\n' "$(git config user.email)" \
        "$(cat ~/.ssh/oddc_release_sk.pub)" >> keys/allowed_signers

Commit the new line on `staging` and promote it to `main` like any other
change. For the first key, also publish its fingerprint
(`ssh-keygen -lf ~/.ssh/oddc_release_sk.pub`) somewhere consumers can
compare it.

Once, per checkout:

    git config gpg.format ssh
    git config user.signingkey ~/.ssh/oddc_release_sk.pub
    git config gpg.ssh.allowedSignersFile "$PWD/keys/allowed_signers"

Per release, after `main` is promoted and pushed:

    git switch main && git pull --ff-only
    nix flake check && go run ./cmd/oddc validate --root .
    git tag -s v2026.10.0 -m "ODDC v2026.10.0"
    git verify-tag v2026.10.0
    git push origin v2026.10.0
