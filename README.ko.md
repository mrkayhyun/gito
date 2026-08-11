# gito

[English](README.md) · 한국어

**gito**는 터미널을 떠나지 않고 자주 쓰는 Git 작업을 대화형 TUI로 처리하는 경량 Git 헬퍼입니다. Go와 [Bubble Tea](https://github.com/charmbracelet/bubbletea) / [Lip Gloss](https://github.com/charmbracelet/lipgloss)로 만들어졌습니다.

```
gito - TUI git helper

Usage:
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

## 프로젝트 개요

gito는 세 개의 계층으로 구성된 단순한 아키텍처를 따릅니다.

```
main.go                 // 서브커맨드 라우팅 + help
└── internal/
    ├── git/            // git CLI를 감싸는 얇은 함수 계층 (부수효과 격리 + 테스트 대상)
    ├── ui/             // 각 명령의 Bubble Tea 모델 (Model/Update/View)
    ├── config/         // gito.json / ~/.config/gito 로딩
    └── style/          // Lip Gloss 공용 스타일 팔레트
```

- **`internal/git`**: 모든 `git` 하위 프로세스 호출을 이 계층에 격리합니다. UI는 여기 함수만 호출하며,
  이 계층은 실제 임시 저장소를 만들어 단위 테스트합니다.
- **`internal/ui`**: 명령마다 `xxxModel`을 두고 `list → detail/input` 패턴(목록 pane + 상세/입력 pane)을
  반복 사용해 일관된 조작감을 제공합니다.
- **설계 원칙**: 상태는 모델에, 부수효과는 git 계층에. UI는 순수 함수(`Update`)로 유지해 테스트와 추론이 쉽습니다.

### 명령별 기능 요약

| 명령 | 설명 | 주요 키 |
|------|------|---------|
| `commit` | 5단계 커밋 마법사(타입→스코프→제목→본문→확인), `gito.json`으로 타입 커스터마이즈 | `y` 커밋, `a` amend, `e` 편집 |
| `log` | 스크롤 가능한 커밋 로그, 상세 diff 보기 | `↑/↓` 이동, `enter` 상세, `g/G` 처음/끝 |
| `branch` | 퍼지 필터 브랜치 전환 + 생성/이름변경/삭제 | `enter` 전환, `^b` 생성, `^r` 이름변경, `^d` 삭제, `^x` 강제삭제 |
| `status` | 스테이징/언스테이징/diff/discard | `space` 토글, `a` 전체 스테이지, `d` diff, `D` discard |
| `stash` | 스태시 목록 관리 | `p` pop, `a` apply, `d` diff, `D` drop |
| `tag` | 태그 생성(경량/주석)/삭제/원격 push | `c` 생성, `p` push, `P` 원격삭제, `D` 삭제 |
| `remote` | 원격 목록, fetch, upstream ahead/behind | `f` fetch, `F` fetch all, `r` 새로고침 |
| `diff` | 두 ref(브랜치/태그) 선택 후 비교 | `enter` 선택(base→target) |
| `reflog` | reflog 탐색 및 커밋 복구(비파괴적: 새 브랜치 생성) | `b` 이 지점으로 브랜치 생성 |
| `blame` | 파일 선택 후 라인별 blame | `enter` blame 보기 |

## 설치

### 1) Homebrew (macOS / Linux)

```bash
brew install mrkayhyun/tap/gito
```

### 2) go install

```bash
go install github.com/mrkayhyun/gito@latest
```

### 3) 설치 스크립트 (zsh)

저장소에서 빌드하고 `~/.local/bin`에 설치한 뒤 `~/.zshrc`의 PATH까지 자동 설정합니다.

```bash
git clone https://github.com/mrkayhyun/gito && cd gito
./install.sh
source ~/.zshrc   # 또는 새 터미널 열기
```

- 설치 위치 변경: `INSTALL_DIR=/usr/local/bin ./install.sh`
- 제거: `./install.sh --uninstall` (바이너리 + `~/.zshrc` 블록 삭제)
- `~/.zshrc`에는 마커 블록만 추가되며, 여러 번 실행해도 중복되지 않습니다(멱등).

### 4) 릴리스 바이너리 / 수동 빌드

[Releases 페이지](https://github.com/mrkayhyun/gito/releases)에서 OS/arch에 맞는 아카이브를
내려받아 압축을 풀고 `gito` 바이너리를 `PATH`에 두어도 됩니다. 직접 빌드하려면:

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

### 종료 키 규칙

- 모든 목록 화면: `esc` 또는 `ctrl+c`로 종료 (필터가 없는 화면은 `q`로도 종료)
- 상세/diff 보기 화면: `q` 또는 `esc`로 목록으로 돌아가기


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

## 개발

```bash
go build ./...     # 빌드
go vet ./...       # 정적 분석
go test ./...      # 테스트 (git 계층은 임시 저장소로 검증)
```

`internal/git`의 함수는 실제 임시 Git 저장소를 생성해 end-to-end로 테스트합니다.
새 git 헬퍼를 추가할 때는 대응하는 테스트를 함께 작성해 주세요.

## 기여

이슈와 풀 리퀘스트를 환영합니다. 자세한 내용은 [CONTRIBUTING.md](CONTRIBUTING.md)를 참고하세요.

보안 문제는 공개 이슈 대신 비공개로 제보해 주세요. [SECURITY.md](SECURITY.md)에 절차가 있습니다.

## 라이선스

[MIT](LICENSE) © mrkayhyun
