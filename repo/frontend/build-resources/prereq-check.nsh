; prereq-check.nsh — NSIS prerequisite check for MedOps Console installer
; Verifies that PostgreSQL 16 is installed before proceeding.

!macro customInstall
  ; Check for PostgreSQL service in Windows services registry
  ReadRegStr $0 HKLM "SYSTEM\CurrentControlSet\Services\postgresql-x64-16" "ImagePath"
  ${If} $0 == ""
    MessageBox MB_YESNO|MB_ICONQUESTION \
      "PostgreSQL 16 does not appear to be installed.$\n$\n\
      MedOps Console requires PostgreSQL 16 to store data locally.$\n$\n\
      Download PostgreSQL 16 from https://www.postgresql.org/download/windows/ \
      then re-run this installer.$\n$\n\
      Click Yes to open the PostgreSQL download page, or No to install anyway." \
      IDNO skip_pg_download
    ExecShell "open" "https://www.postgresql.org/download/windows/"
    Abort "Installation aborted — please install PostgreSQL 16 first."
    skip_pg_download:
  ${EndIf}
!macroend

!macro customUnInstall
  ; Nothing special needed on uninstall
!macroend
