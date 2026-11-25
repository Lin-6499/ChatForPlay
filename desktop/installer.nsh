; Keep installation scoped to the current Windows user, with a selectable folder.
!macro customInstallMode
  StrCpy $isForceCurrentInstall "1"
!macroend

!macro customFinishPage
  ; The source launcher can request an unchecked Run box, while the user retains
  ; control over whether to open LinChat on the installer's final page.
  !define MUI_FINISHPAGE_RUN
  !define MUI_FINISHPAGE_RUN_FUNCTION "LinChatStartApp"
  !define MUI_PAGE_CUSTOMFUNCTION_SHOW "LinChatFinishShow"
  !insertmacro MUI_PAGE_FINISH

  Function LinChatFinishShow
    ClearErrors
    ${GetOptions} $CMDLINE "/NoLaunch" $0
    ${IfNot} ${Errors}
      SendMessage $mui.FinishPage.Run ${BM_SETCHECK} ${BST_UNCHECKED} 0
    ${EndIf}
  FunctionEnd

  Function LinChatStartApp
    ${If} ${isUpdated}
      StrCpy $1 "--updated"
    ${Else}
      StrCpy $1 ""
    ${EndIf}
    ${StdUtils.ExecShellAsUser} $0 "$launchLink" "open" "$1"
  FunctionEnd
!macroend
