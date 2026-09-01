# Release policy

The repository was bootstrapped from `gooo-repository-bootstrap@v0.1.1` under
the sole `BOOTSTRAP_EXCEPTION`. The bootstrap commit is recorded by
`contracts/bootstrap-lock-v1.json`; post-bootstrap direct-main implementation
commits are zero, and only one implementation pull request may be open.

GitHub Actions is the success authority. The release workflow requires the
exact merged `main` SHA, refuses a tag or release that already exists, creates
one annotated tag, publishes six assets, and verifies the release API reports
`immutable=true`. It also verifies the annotated tag object points to the
requested commit and checks each uploaded asset's name, size, and SHA-256.

Failed runs, existing tags, and non-immutable releases are historical
evidence. They are never deleted, rewritten, or overwritten; a recovery uses a
new unused version.

