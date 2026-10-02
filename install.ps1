# Memasang gonsu di Windows (PowerShell):
#
#   irm https://raw.githubusercontent.com/gonsutrijayautama/gonsu-cli/main/install.ps1 | iex
#
# Yang dikerjakan: mengunduh binary rilis untuk mesin ini, mencocokkan
# SHA-256-nya dengan SHA256SUMS rilis yang sama, menaruhnya di
# %LOCALAPPDATA%\Programs\gonsu, lalu menambahkan folder itu ke PATH pengguna.
# Tidak butuh hak administrator.
#
# Pengaturan lewat environment:
#   GONSU_VERSION        versi yang dipasang, mis. 0.1.0 (bawaan: rilis terbaru)
#   GONSU_INSTALL_DIR    folder tujuan
#   GONSU_DOWNLOAD_BASE  alamat atau folder berisi berkas rilis, menggantikan
#                        GitHub Releases - untuk menguji skrip ini

# Di dalam scriptblock: `irm | iex` menjalankan skrip di sesi pemakai, dan
# pengaturan di bawah tidak boleh tertinggal di sesi itu.
& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue'
    # Windows PowerShell 5.1 masih bisa memulai dengan TLS 1.0.
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    $repo = 'gonsutrijayautama/gonsu-cli'

    $machine = $env:PROCESSOR_ARCHITEW6432
    if (-not $machine) { $machine = $env:PROCESSOR_ARCHITECTURE }
    switch ($machine) {
        'AMD64' { $arch = 'amd64' }
        'ARM64' { $arch = 'arm64' }
        default { throw "gonsu: arsitektur $machine belum didukung" }
    }

    $version = $env:GONSU_VERSION
    $dir = $env:GONSU_INSTALL_DIR
    if (-not $dir) { $dir = Join-Path $env:LOCALAPPDATA 'Programs\gonsu' }
    $asset = "gonsu_windows_$arch.zip"
    if ($env:GONSU_DOWNLOAD_BASE) {
        $base = $env:GONSU_DOWNLOAD_BASE
    } elseif ($version) {
        $base = "https://github.com/$repo/releases/download/v$($version.TrimStart('v'))"
    } else {
        $base = "https://github.com/$repo/releases/latest/download"
    }

    # Sumber berupa alamat, atau berkas di folder lokal.
    function Get-ReleaseFile([string]$Name, [string]$Destination) {
        if (Test-Path -LiteralPath $base -PathType Container) {
            Copy-Item -LiteralPath (Join-Path $base $Name) -Destination $Destination
        } else {
            Invoke-WebRequest -UseBasicParsing -Uri "$base/$Name" -OutFile $Destination
        }
    }

    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("gonsu-" + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "Mengunduh $asset..."
        $archive = Join-Path $tmp $asset
        $sums = Join-Path $tmp 'SHA256SUMS'
        Get-ReleaseFile $asset $archive
        Get-ReleaseFile 'SHA256SUMS' $sums

        # Unduhan yang rusak atau tertukar tidak pernah dipasang.
        $expected = $null
        foreach ($line in Get-Content -LiteralPath $sums) {
            $parts = $line -split '\s+', 2
            # "<hash>  <nama>", atau "<hash> *<nama>" bila dibuat di Windows.
            if ($parts.Count -eq 2 -and $parts[1].Trim().TrimStart('*') -eq $asset) { $expected = $parts[0].ToLower() }
        }
        if (-not $expected) { throw "gonsu: SHA256SUMS tidak memuat $asset" }
        $actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLower()
        if ($actual -ne $expected) { throw "gonsu: SHA-256 $asset tidak cocok dengan SHA256SUMS; unduhan tidak dipasang" }

        Expand-Archive -LiteralPath $archive -DestinationPath $tmp -Force
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        Copy-Item -LiteralPath (Join-Path $tmp 'gonsu.exe') -Destination (Join-Path $dir 'gonsu.exe') -Force
    } finally {
        Remove-Item -LiteralPath $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    $exe = Join-Path $dir 'gonsu.exe'
    Write-Host "Terpasang: $(& $exe version) di $exe"

    # PATH pengguna, bukan PATH mesin: tidak butuh hak administrator.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $entries = @()
    if ($userPath) { $entries = $userPath -split ';' | Where-Object { $_ } }
    if ($entries -notcontains $dir) {
        [Environment]::SetEnvironmentVariable('Path', (($entries + $dir) -join ';'), 'User')
        $env:Path = "$env:Path;$dir"
        Write-Host "$dir ditambahkan ke PATH pengguna. Terminal yang sudah terbuka perlu dibuka ulang."
    }

    Write-Host ''
    Write-Host 'Berikutnya: gonsu new <kode-produk>'
    Write-Host 'Starter kit GONSU privat; lihat bagian Memasang di README untuk akses git-nya:'
    Write-Host "  https://github.com/$repo#memasang"
}
