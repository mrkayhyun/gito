# gito

[English](README.md) · 한국어

**gito**는 터미널을 떠나지 않고 자주 쓰는 Git 작업을 대화형 TUI로 처리하는 경량 Git 헬퍼입니다. Go와 [Bubble Tea](https://github.com/charmbracelet/bubbletea) / [Lip Gloss](https://github.com/charmbracelet/lipgloss)로 만들어졌습니다.

---

## 이 프로젝트를 만드는 목적

일상적인 Git 작업 대부분은 **"기억해야 할 명령과 플래그"** 때문에 느려집니다.
`git rebase -i`, `git reflog`, `git commit --amend`, `git push origin --delete <tag>` 같은
명령들은 강력하지만, 매번 정확한 문법을 떠올려야 하고 실수하면 되돌리기 어렵습니다.

gito의 목적은 이 마찰을 없애는 것입니다.

- **탐색 가능한 인터페이스**: 명령을 외우는 대신 목록에서 고르고 키 힌트를 따라갑니다.
- **안전한 기본값**: 파괴적인 작업(브랜치 삭제, 태그 원격 삭제 등)은 항상 확인 단계를 거칩니다.
- **터미널 네이티브**: 별도 GUI 앱 없이, SSH 세션이나 원격 서버에서도 그대로 동작합니다.
- **빠른 실행**: 단일 정적 바이너리로 배포되며 의존성이 없습니다.

## 왜 또 다른 Git 도구인가

이미 `lazygit`, `tig`, `gitui` 같은 훌륭한 도구가 있습니다. gito는 그것들을 대체하기보다,
다음과 같은 다른 지향점을 가집니다.

1. **명령 단위의 작은 도구 모음** — 하나의 거대한 대시보드가 아니라, `gito commit`, `gito tag`처럼
   목적이 분명한 하위 명령으로 나뉩니다. 필요한 화면에 바로 진입하고 끝나면 셸로 돌아옵니다.
2. **읽기 쉬운 최소 코드베이스** — 각 명령이 독립된 Bubble Tea 모델 하나로 구현되어,
   Go/TUI 입문자가 코드를 읽고 기여하기 쉽도록 설계했습니다. (학습·교육용으로도 적합)
3. **설정 기반 커밋 컨벤션** — 팀의 커밋 타입 규칙을 `gito.json`으로 정의해 커밋 마법사에 그대로 반영합니다.

## 명령

```
gito commit    Interactive commit wizard (5-step, config-driven types)
gito log       Scrollable log viewer  (↑/↓ navigate, enter: detail)
gito branch    Fuzzy branch switcher / create / rename / delete
gito status    Interactive stage / unstage / diff / discard
gito stash     Stash list  (pop / apply / show diff / drop)
gito tag       Tag manager (create / delete / push / show diff)
gito remote    Remote list (fetch / ahead-behind status)
gito diff      Compare two refs (branch/tag) and view the diff
gito reflog    Browse reflog and recover commits into a new branch
gito blame     Pick a file and view line-by-line blame
```

| 명령 | 설명 | 주요 키 |
|------|------|---------|
| `commit` | 5단계 커밋 마법사(타입→스코프→제목→본문→확인), 작성 중인 메시지 미리보기 제공, `gito.json`으로 타입 커스터마이즈 | `enter` 다음, `esc` 이전, `y` 커밋, `a` amend, `e` 편집 |
| `log` | 스크롤 가능한 커밋 로그, 상세 diff 보기 | `↑/↓` `j/k` 이동, `g/G` 처음/끝, `enter` 상세 |
| `branch` | 퍼지 필터 브랜치 전환 + 생성/이름변경/삭제 | 입력하면 필터, `↑/↓` `^p/^n` 이동, `enter` 전환, `^b` 생성, `^r` 이름변경, `^d` 삭제, `^x` 강제삭제 |
| `status` | 스테이징/언스테이징/diff/discard | `space` 토글, `a` 전체 스테이지, `d` diff, `D` discard |
| `stash` | 스태시 목록 관리 | `enter` / `p` pop, `a` apply, `d` diff, `D` drop |
| `tag` | 태그 생성(경량/주석)/삭제/원격 push | `enter` / `d` 상세, `c` 생성, `p` push, `P` 원격삭제, `D` 삭제 |
| `remote` | 원격 목록, fetch, upstream ahead/behind | `f` fetch, `F` fetch all, `r` 새로고침 |
| `diff` | 두 ref(브랜치/태그) 선택 후 비교 | `enter` 선택(base→target), `esc` 한 단계 뒤로 |
| `reflog` | reflog 탐색 및 커밋 복구(비파괴적: 새 브랜치 생성) | `g/G` 처음/끝, `b` 이 지점으로 브랜치 생성 |
| `blame` | 파일 선택 후 라인별 blame | 입력하면 필터, `↑/↓` `^p/^n` 이동, `enter` blame 보기 |

모든 화면은 터미널 폭에 맞춰 줄어드는 푸터에 자기 키 힌트를 표시하며, 오버레이가 있는 화면에서는 `?`로
전체 키 목록을 볼 수 있습니다([키 조작과 도움말](#키-조작과-도움말) 참고).

## 설치

### 1) 설치 스크립트 (권장, zsh)

저장소에서 빌드하고 `~/.local/bin`에 설치한 뒤 `~/.zshrc`의 PATH까지 자동 설정합니다.

```bash
git clone https://github.com/mrkayhyun/gito && cd gito
./install.sh
source ~/.zshrc   # 또는 새 터미널 열기
```

- 설치 위치 변경: `INSTALL_DIR=/usr/local/bin ./install.sh`
- 제거: `./install.sh --uninstall` (바이너리 + `~/.zshrc` 블록 삭제)
- `~/.zshrc`에는 마커 블록만 추가되며, 여러 번 실행해도 중복되지 않습니다(멱등).

### 2) 수동 빌드

```bash
# 소스에서 빌드
git clone https://github.com/mrkayhyun/gito && cd gito
go build -o gito .

# 또는 go install
go install .
```

빌드 산출물은 의존성 없는 단일 바이너리입니다. `PATH`에 두고 사용하세요.
버전을 새기려면: `go build -ldflags "-X main.version=v1.0.0" -o gito .`

## 사용법

Git 저장소 안에서 원하는 하위 명령을 실행합니다.

```bash
gito status     # 변경 파일 스테이징/확인
gito commit     # 대화형 커밋 작성
gito log        # 히스토리 탐색
gito diff       # 두 브랜치/태그 비교
```

### 런처 메뉴

명령 이름을 외우지 않아도 됩니다. 인자 없이 `gito`만 실행하면 **대화형 런처 메뉴**가 열리고,
화살표(`↑/↓`)로 명령을 고르거나 숫자(`1`~`9`, `0`은 10번째)로 바로 선택해 실행할 수 있습니다.

```bash
gito            # 런처 메뉴 열기
gito menu       # (명시적으로) 런처 메뉴 열기
```

### 기타 명령

```bash
gito help       # 전체 명령 도움말 (gito -h / --help 동일)
gito version    # 버전 출력 (gito -v / --version 동일)
```

git 저장소가 아닌 곳에서 명령을 실행하면 친절한 안내 메시지를 출력하고 종료합니다.

### 키 조작과 도움말

- 목록 화면: `esc` 또는 `ctrl+c`로 종료 (텍스트 필터가 없는 화면은 `q`로도 종료)
- 상세/diff 보기 화면: `↑/↓` `j/k` 스크롤, `PgUp/PgDn` 페이지 이동, `q` 또는 `esc`로 목록으로 돌아가기
- 확인 프롬프트: `y`는 실행, 그 외 키는 모두 취소
- `?`는 현재 화면의 전체 키 목록을 오버레이로 보여줍니다. `status`, `log`, `stash`, `tag`,
  `diff`의 ref 선택 화면, `remote` 목록, `reflog` 목록에 있습니다. `branch`와 `blame`은 입력한 글자가
  필터로 들어가고 런처와 `commit`은 단일 키를 그대로 받으므로 `?`를 두지 않고 힌트를 푸터에만 표시합니다.
  확인 프롬프트가 떠 있는 동안에는 `?`도 무시됩니다. 표를 다 담을 수 없을 만큼 짧은 터미널에서는
  오버레이가 잘려 나가는 대신 뒤쪽 힌트를 접고 `+N개 더` 표시로 몇 개가 숨었는지 알려줍니다.


## 설정 (선택)

커밋 타입을 팀 규칙에 맞게 정의할 수 있습니다. 다음 순서로 설정을 찾습니다.

1. `./gito.json` (프로젝트 루트, 우선)
2. `~/.config/gito/config.json`

```json
{
  "commit_types": [
    {"key": "feat",  "label": "feat      새로운 기능"},
    {"key": "fix",   "label": "fix       버그 수정"},
    {"key": "docs",  "label": "docs      문서 변경"}
  ]
}
```

설정이 없으면 Conventional Commits 기반 기본값(feat/fix/docs/style/refactor/test/chore)을 사용합니다.

## 터미널 호환성

로컬 터미널에서든 원격 서버의 SSH 세션에서든 같은 화면을 보여주는 것을 목표로 합니다.

- **밝은/어두운 배경**: 모든 색은 밝은 배경과 어두운 배경 값을 따로 가진 Lip Gloss 적응형 색이며,
  터미널이 지원하는 범위로 자동 축소됩니다. `NO_COLOR`도 존중합니다.
- **UTF-8이 아닌 터미널**: 커서, 체크 표시, 화살표, 박스 테두리, 런처 아이콘은 글리프 테이블에서 가져오며,
  `LC_ALL` / `LC_CTYPE` / `LANG`이 UTF-8을 알리지 않으면 순수 ASCII로 자동 대체됩니다.
  `GITO_ASCII=1`(`true` / `yes` / `y` / `on`도 가능)로 언제든 ASCII 테이블을 강제할 수 있습니다.
- **모든 터미널 크기**: 모든 화면이 크기 변경에 반응합니다. 목록은 스크롤하며 커서를 항상 보이게 유지하고,
  키 힌트는 한 줄에 맞게 줄어들고, 긴 줄은 줄바꿈 대신 잘립니다. 좁거나 낮은 터미널에서는 화면이 깨지지 않고
  기능이 축소된 형태로 표시됩니다. 레이아웃 계산의 하한은 **가로 20칸 × 세로 6줄**입니다. 그보다 작은
  터미널에서도 20x6 기준으로 그리기 때문에 더 좁으면 줄바꿈이, 더 낮으면 스크롤이 생깁니다.
  하한 이상의 크기에서는 줄바꿈 없이 화면이 맞춰집니다.

## 아키텍처

gito는 세 개의 계층으로 구성된 단순한 아키텍처를 따릅니다.

```
main.go                 // 서브커맨드 라우팅 + help
└── internal/
    ├── git/            // git CLI를 감싸는 얇은 함수 계층 (부수효과 격리 + 테스트 대상)
    ├── ui/             // 각 명령의 Bubble Tea 모델 (Model/Update/View)
    │   └── chrome.go   // 공용 크롬: 레이아웃, 헤더, 힌트 푸터, 도움말 오버레이, 행/스크롤
    ├── config/         // gito.json / ~/.config/gito 로딩
    ├── i18n/           // 메시지 카탈로그(en/ko/ja/zh) + 로케일 감지
    └── style/          // 의미 기반 적응형 테마, 글리프 테이블, ANSI 인식 폭 계산 헬퍼
```

- **`internal/git`**: 모든 `git` 하위 프로세스 호출을 이 계층에 격리합니다. UI는 여기 함수만 호출하며,
  이 계층은 실제 임시 저장소를 만들어 단위 테스트합니다.
- **`internal/ui`**: 명령마다 `xxxModel`을 두고 `list → detail/input` 패턴(목록 pane + 상세/입력 pane)을
  반복 사용해 일관된 조작감을 제공합니다. 열 개 명령 모델 모두 `internal/ui/chrome.go`를 통해 렌더링하므로
  본문 높이 계산, 한 줄 헤더, 폭에 맞춰 줄어드는 키 힌트 푸터, `?` 오버레이, 메시지/확인 배너,
  선택 행 강조, 목록 스크롤이 화면마다 반복되지 않고 한 곳에만 존재합니다.
- **`internal/style`**: 색을 hex 값이 아니라 역할(`Hash`, `MetaDim`, `Staged`, `Unstaged`,
  `DangerBar` 등)로 노출하며, Unicode/ASCII 글리프 테이블과 ANSI 인식 폭 계산 헬퍼를 함께 제공합니다.
  덕분에 `git`이 이미 색을 입혀 내려주는 diff 출력도 레이아웃을 깨뜨리지 않습니다.
- **설계 원칙**: 상태는 모델에, 부수효과는 git 계층에. UI는 순수 함수(`Update`)로 유지해 테스트와 추론이 쉽습니다.

## 개발

```bash
go build ./...     # 빌드
go vet ./...       # 정적 분석
go test ./...      # 테스트 (git 계층은 임시 저장소로 검증)
```

`internal/git`의 함수는 실제 임시 Git 저장소를 생성해 end-to-end로 테스트합니다.
새 git 헬퍼를 추가할 때는 대응하는 테스트를 함께 작성해 주세요.

## 기여

새 명령을 추가하려면:

1. `internal/git`에 필요한 git 래퍼 함수 + 테스트를 추가합니다.
2. `internal/ui`에 `xxxModel`(Model/Update/View)과 `RunXxx()`를 작성합니다. 기존 `stash.go`/`tag.go`가 좋은 템플릿입니다.
3. `main.go`의 switch와 help 텍스트에 명령을 등록합니다.
4. `go build ./... && go vet ./... && go test ./...`가 통과하는지 확인합니다.

## 라이선스

[MIT](LICENSE) © mrkayhyun
