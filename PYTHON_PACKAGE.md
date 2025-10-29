# Python Package Guide

This guide explains how to build, test, and publish the agentapi Python package.

## Package Structure

```
agentapi/
├── src/
│   └── agentapi/
│       ├── __init__.py       # Package initialization
│       ├── __main__.py       # Allows 'python -m agentapi'
│       ├── cli.py            # CLI entry point
│       └── download.py       # Binary download logic
├── pyproject.toml            # Package metadata
└── MANIFEST.in              # Additional files to include
```

## How It Works

1. When installed via `pip install agentapi`, the package creates an `agentapi` command
2. On first run, it downloads the appropriate binary for the user's platform from GitHub releases
3. The binary is stored in the package directory and reused for subsequent runs
4. All CLI arguments are passed directly to the binary

## Development

### Install in development mode

```bash
pip install -e .
```

This allows you to make changes and test them immediately.

### Install with dev dependencies

```bash
pip install -e ".[dev]"
```

## Testing Locally

1. Install the package in development mode:
   ```bash
   pip install -e .
   ```

2. Test the CLI:
   ```bash
   agentapi --help
   agentapi --version
   ```

3. Test running as a module:
   ```bash
   python -m agentapi --help
   ```

4. Test the server:
   ```bash
   agentapi server -- claude
   ```

## Building

Build the package for distribution:

```bash
# Install build tools
pip install build

# Build the package
python -m build
```

This creates:
- `dist/agentapi-0.10.1-py3-none-any.whl` (wheel)
- `dist/agentapi-0.10.1.tar.gz` (source distribution)

## Publishing to PyPI

### Test on TestPyPI first

1. Create an account on [TestPyPI](https://test.pypi.org/account/register/)

2. Install twine:
   ```bash
   pip install twine
   ```

3. Upload to TestPyPI:
   ```bash
   python -m twine upload --repository testpypi dist/*
   ```

4. Test installation from TestPyPI:
   ```bash
   pip install --index-url https://test.pypi.org/simple/ agentapi
   ```

### Publish to PyPI

1. Create an account on [PyPI](https://pypi.org/account/register/)

2. Upload to PyPI:
   ```bash
   python -m twine upload dist/*
   ```

3. Users can now install via:
   ```bash
   pip install agentapi
   ```

## Automated Publishing with GitHub Actions

Create `.github/workflows/publish-pypi.yml`:

```yaml
name: Publish to PyPI

on:
  release:
    types: [published]

jobs:
  build-and-publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.11'

      - name: Install dependencies
        run: |
          python -m pip install --upgrade pip
          pip install build twine

      - name: Build package
        run: python -m build

      - name: Publish to PyPI
        env:
          TWINE_USERNAME: __token__
          TWINE_PASSWORD: ${{ secrets.PYPI_API_TOKEN }}
        run: twine upload dist/*
```

Then add your PyPI API token as a secret named `PYPI_API_TOKEN` in your GitHub repository settings.

## Version Management

Update the version in both:
1. `pyproject.toml` - the `version` field
2. `src/agentapi/__init__.py` - the `__version__` variable
3. `src/agentapi/download.py` - the `VERSION` variable

These should match the GitHub release tag (without the 'v' prefix).

## Platform Support

The package automatically detects the platform and downloads the appropriate binary:

- **Linux**: `agentapi-linux-amd64` or `agentapi-linux-arm64`
- **macOS**: `agentapi-darwin-amd64` or `agentapi-darwin-arm64`
- **Windows**: `agentapi-windows-amd64.exe` or `agentapi-windows-arm64.exe`

## Troubleshooting

### Binary not found
If users report issues with binary downloads, they can:
1. Manually download from GitHub releases
2. Place the binary in the package directory
3. Or set force download: `python -c "from agentapi.download import download_binary; download_binary(force=True)"`

### Import errors
Make sure the package is installed: `pip install -e .` for development or `pip install agentapi` for production.
