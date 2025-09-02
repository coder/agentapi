# Information for Maintainers

## Release Process

Before performing a release, perform a local "smoke-test".
If everything seems OK, you can proceed to do the following:

1. Update the version string in `internal/version/version.go` and run `make gen`.
2. Add details in `CHANGELOG.md` on what changed.
3. Create a PR with the subject `chore: update version to X.Y.Z`
4. Once the above PR is approved and merged, create a new git tag `vX.Y.Z` pointing to the commit of the above PR merged to `main`:

   ```shell
     # Ensure your local copy is up to date with main. Be sure to stash any changes first.
     git fetch origin
     git reset --hard origin/main
     # Fetch existing tags first!
     git fetch --tags
     git tag -a vX.Y.Z -m 'vX.Y.Z'
   ```

5. Push the tag:

   ```shell
      git push origin tag vX.Y.Z
   ```

6. Visit `https://github.com/coder/agentapi/releases/tag/vX.Y.Z` and "Create release from tag".

   - Select the tag you pushed previously.
   - Select the previous tag and "Generate release notes". Amend as required.
   - **IMPORTANT:** un-check "Set as latest release" and check "Set as a pre-release".
   - Click "Publish Release". This will trigger a "Build Release Binaries" CI job.

7. Visit `https://github.com/coder/agentapi/actions/workflows/release.yml` and monitor the status of the job that was created in the previous step. This will upload the built assets to the corresponding release.

8. Once the updated assets are released, you can now visit `https://github.com/coder/agentapi/releases/tag/vX.Y.Z`, click "Edit" (✎), and check "Set as latest release".
