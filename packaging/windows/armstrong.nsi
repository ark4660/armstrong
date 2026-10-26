; Armstrong installer for Windows (NSIS 3). The release workflow builds it:
;
;   makensis -DVERSION=0.3.0 -DDIST=<dist> -DOUT=<dist>/ArmstrongSetup.exe armstrong.nsi
;
; Installs per user to %LOCALAPPDATA%\Programs\Armstrong (no admin prompt, and
; the launcher can update itself there), with Start Menu and optional desktop
; shortcuts, "armstrong" on the user PATH, and an entry in Installed apps.
; One installer carries both the x64 and ARM64 launchers and installs the one
; that matches the PC.

!ifndef VERSION
	!error "pass -DVERSION=x.y.z"
!endif
; Windows file versions are four numbers: 0.3.0-beta.1 becomes 0.3.0.0.
!ifndef VIVERSION
	!define VIVERSION "${VERSION}.0"
!endif
!ifndef DIST
	!define DIST "..\..\dist"
!endif
!ifndef OUT
	!define OUT "ArmstrongSetup.exe"
!endif

!define NAME "Armstrong"
!define EXE "armstrong.exe"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Armstrong"
!define ICON "..\..\assets\armstrong.ico"

Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user
ManifestDPIAware true

Name "${NAME}"
OutFile "${OUT}"
InstallDir "$LOCALAPPDATA\Programs\Armstrong"
InstallDirRegKey HKCU "${UNINSTALL_KEY}" "InstallLocation"
BrandingText "${NAME} ${VERSION}"

VIProductVersion "${VIVERSION}"
VIAddVersionKey "ProductName" "${NAME}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "${NAME} installer"
VIAddVersionKey "LegalCopyright" "Armstrong contributors"

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"

!define MUI_ICON "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_ABORTWARNING
!define MUI_WELCOMEFINISHPAGE_BITMAP "sidebar.bmp"
!define MUI_UNWELCOMEFINISHPAGE_BITMAP "sidebar.bmp"
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_RIGHT
!define MUI_HEADERIMAGE_BITMAP "header.bmp"
!define MUI_HEADERIMAGE_UNBITMAP "header.bmp"

!define MUI_WELCOMEPAGE_TEXT "This will install ${NAME} ${VERSION}, a Minecraft launcher and performance client.$\r$\n$\r$\nNo administrator rights are needed. ${NAME} keeps itself up to date after this.$\r$\n$\r$\nClick Next to continue."
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN
!define MUI_FINISHPAGE_RUN_TEXT "Start ${NAME}"
!define MUI_FINISHPAGE_RUN_FUNCTION StartArmstrong
; The "show readme" checkbox doubles as the desktop shortcut option.
!define MUI_FINISHPAGE_SHOWREADME
!define MUI_FINISHPAGE_SHOWREADME_TEXT "Create a desktop shortcut"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION DesktopShortcut
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; Adds or removes the install folder on the user PATH. PowerShell does the
; list editing and tells running programs the PATH changed; the folder is
; passed through the environment so no quoting can break it.
!macro UserPath ACTION
	System::Call 'Kernel32::SetEnvironmentVariable(t "ARMSTRONG_DIR", t "$INSTDIR")'
	!if "${ACTION}" == "add"
		nsExec::Exec `powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$$d = $$env:ARMSTRONG_DIR; $$p = [Environment]::GetEnvironmentVariable('Path', 'User'); $$parts = @($$p -split ';' | Where-Object { $$_ }); if ($$parts -notcontains $$d) { [Environment]::SetEnvironmentVariable('Path', (@($$parts) + $$d) -join ';', 'User') }"`
	!else
		nsExec::Exec `powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$$d = $$env:ARMSTRONG_DIR; $$p = [Environment]::GetEnvironmentVariable('Path', 'User'); [Environment]::SetEnvironmentVariable('Path', (@($$p -split ';' | Where-Object { $$_ -and $$_.TrimEnd('\') -ne $$d.TrimEnd('\') }) -join ';'), 'User')"`
	!endif
	Pop $0
!macroend

Function .onInit
	${IfNot} ${RunningX64}
		MessageBox MB_ICONSTOP "${NAME} needs 64-bit Windows." /SD IDOK
		Abort
	${EndIf}
FunctionEnd

Section "Install"
	SetOutPath "$INSTDIR"
	; Left behind by the launcher's self-update.
	Delete "$INSTDIR\${EXE}.old"

	${If} ${IsNativeARM64}
		File "/oname=${EXE}" "${DIST}\armstrong-windows-arm64.exe"
	${Else}
		File "/oname=${EXE}" "${DIST}\armstrong-windows-amd64.exe"
	${EndIf}
	File "/oname=armstrong.ico" "${ICON}"
	WriteUninstaller "$INSTDIR\Uninstall.exe"

	SetOutPath "$PROFILE"
	CreateShortcut "$SMPROGRAMS\${NAME}.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\armstrong.ico" 0 SW_SHOWNORMAL "" "Minecraft launcher and performance client"

	!insertmacro UserPath add

	WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "${NAME}"
	WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${VERSION}"
	WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher" "Armstrong"
	WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\armstrong.ico"
	WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
	WriteRegStr HKCU "${UNINSTALL_KEY}" "URLInfoAbout" "https://github.com/ark4660/armstrong"
	WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
	WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
	WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
	WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
	WriteRegDWORD HKCU "${UNINSTALL_KEY}" "EstimatedSize" 12000
SectionEnd

Function StartArmstrong
	SetOutPath "$PROFILE"
	Exec '"$INSTDIR\${EXE}"'
FunctionEnd

Function DesktopShortcut
	SetOutPath "$PROFILE"
	CreateShortcut "$DESKTOP\${NAME}.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\armstrong.ico" 0
FunctionEnd

Section "Uninstall"
	Delete "$INSTDIR\${EXE}"
	Delete "$INSTDIR\${EXE}.old"
	Delete "$INSTDIR\${EXE}.new"
	Delete "$INSTDIR\armstrong.ico"
	Delete "$INSTDIR\Uninstall.exe"
	RMDir "$INSTDIR"
	Delete "$SMPROGRAMS\${NAME}.lnk"
	Delete "$DESKTOP\${NAME}.lnk"
	!insertmacro UserPath remove
	DeleteRegKey HKCU "${UNINSTALL_KEY}"

	; Instances, accounts and settings: kept unless the player says otherwise.
	MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2 "Also delete your Armstrong instances, worlds, accounts and settings?$\r$\n$\r$\n($APPDATA\armstrong)" /SD IDNO IDNO keep
	RMDir /r "$APPDATA\armstrong"
	keep:
SectionEnd
