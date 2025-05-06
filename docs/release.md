# Release Process for tsgrok

This document outlines the steps to create a new release for the `tsgrok` project using GoReleaser.

## Prerequisites

1.  Ensure `goreleaser` is installed and configured correctly.
2.  Ensure you have rights to push to the repository and create releases on GitHub.
3.  Make sure your GPG key is configured for signing.

## Steps to Create a Release


1.  **Determine the New Version:**
    *   Decide on the new version number (e.g., `v0.1.0`, `v1.2.3`).

2.  **Create an Annotated Git Tag:**
    *   Create an annotated and GPG-signed tag for the new version. Replace `vX.Y.Z` with your chosen version number and provide a meaningful tag message.
        ```bash
        git tag -a vX.Y.Z -m "Release version vX.Y.Z"
        ```
3.  **Push Changes and Tag to GitHub:**
    *   Push your commits and the new tag to the remote repository:
        ```bash
        git push origin main # Or your primary branch
        git push origin vX.Y.Z
        ```
        (Replace `vX.Y.Z` with your actual tag)
        Alternatively, to push all tags:
        ```bash
        git push --tags
        ```

4.  **Run GoReleaser:**
    *   Execute the `goreleaser release` command. This will build the project, create the release on GitHub, and upload the artifacts.
        ```bash
        goreleaser release --clean
        ```
    *   The `--clean` flag ensures that the `dist` directory is cleaned before building, which is good practice.
    *   If you have a CI/CD pipeline set up, this step might be triggered automatically when a new tag is pushed.

5.  **Verify Release:**
    *   Go to the releases page on GitHub for the project and verify that the new release is present with all artifacts and release notes (if `goreleaser` is configured to generate them from your changelog).
