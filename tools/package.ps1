param([string]$OutputPath)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression,System.IO.Compression.FileSystem
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $OutputPath) { $OutputPath = Join-Path ([IO.Directory]::GetParent($projectRoot).FullName) 'StudentRecordSystem_Complete.zip' }
$archivePath = [IO.Path]::GetFullPath($OutputPath)
if (Test-Path -LiteralPath $archivePath) { throw "Archive already exists: $archivePath. Choose another -OutputPath." }
$archive = [IO.Compression.ZipFile]::Open($archivePath, [IO.Compression.ZipArchiveMode]::Create)
try {
  foreach ($file in Get-ChildItem -LiteralPath $projectRoot -File -Recurse -Force) {
    $relative = $file.FullName.Substring($projectRoot.Length + 1).Replace('\','/')
    if ($relative -match '^data/' -and $relative -ne 'data/.gitkeep') { continue }
    if ($relative -match '(^|/)(\.git|node_modules|dist)/|\.(db|db-wal|db-shm|log|out|test)$|(^|/)\.env$') { continue }
    if ($file.Extension -eq '.exe' -and $relative -notin @('student-records.exe','language-demo.exe')) { continue }
    [IO.Compression.ZipFileExtensions]::CreateEntryFromFile($archive, $file.FullName, ('StudentRecordSystem/' + $relative), [IO.Compression.CompressionLevel]::Optimal) | Out-Null
  }
} finally { $archive.Dispose() }
Get-Item -LiteralPath $archivePath | Select-Object FullName,Length
