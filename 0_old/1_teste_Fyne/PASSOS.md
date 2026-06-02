# Teste Fyne — passos para o Alexandre

Objetivo: confirmar que o Fyne compila e corre num PC da escola antes de migrarmos o cliente GUI todo. Se falhar lá, ajustamos o plano sem termos perdido horas.

Corre tudo num PC da escola (Linux), não na tua máquina pessoal.

---

## 1. Confirmar que há Go instalado

```bash
go version
```

- Se devolver algo tipo `go version go1.22.x linux/amd64` → segue para o passo 2.
- Se devolver `command not found` → diz-me, sem Go nada disto avança.
- Se devolver uma versão antiga, como `go1.18.1`, o problema costuma ser o `PATH` a apontar para o Go do sistema (`/usr/bin/go`) em vez do Go instalado na tua conta.
- A correção que funcionou foi instalar o Go 1.22.5 em `~/.local/go` e pô-lo primeiro no `PATH`.
- Estes foram os comandos que usei sem `sudo`:

```bash
# 1) Pick a version from https://go.dev/dl
#    Example below uses go1.22.5; replace if needed.

cd /tmp
wget https://go.dev/dl/go1.22.5.linux-amd64.tar.gz

# 2) Install it under your home directory
mkdir -p "$HOME/.local"
tar -C "$HOME/.local" -xzf go1.22.5.linux-amd64.tar.gz

# 3) Use this Go first in your shell
export PATH="$HOME/.local/go/bin:$PATH"

# 4) Check it worked
go version
```

No nosso caso, isto fez o terminal passar de `/usr/bin/go` para `~/.local/go/bin/go`, e o `go mod tidy` deixou de reclamar da versão mínima.

## 2. Confirmar que há compilador C

```bash
clang --version
```

(ou `gcc --version` se o `clang` não existir)

O Fyne usa CGO, por isso precisa de um compilador C. Já sabemos que o clang está disponível na escola, mas convém confirmar.

## 3. Ir para a pasta do teste

A partir da raiz do projeto:

```bash
cd testeFyne
```

## 4. Descarregar as dependências do Fyne

```bash
go mod tidy
```

Vai buscar o Fyne e dependências transitivas. Demora 1-2 minutos na primeira vez. Não precisa de root, é tudo no espaço de utilizador (`~/go/pkg/...`).

## 5. Compilar e correr

```bash
go run .
```

### Resultado esperado

Abre uma janela pequena com:
- O texto "If you can read this, Fyne works on this machine."
- Um botão "Click me"

Clica no botão → o texto muda. Se isto acontecer, o teste passou.

---

## Se falhar

Antes de mais, vale a pena tentares diagnosticar tu sem esperar — a maior parte dos erros tem mais que uma solução, podes ir tentando.

### Se aparecer erro de UTF-8 / go-gl corrompido

Se o `go run .` falhar com algo do género `invalid UTF-8 encoding` dentro de `github.com/go-gl/gl`, o problema foi o cache de módulos do Go. A correção que resolveu foi limpar o cache e voltar a descarregar as dependências:

```bash
go clean -modcache
go mod tidy
go run .
```

### Diagnóstico rápido (corre tudo, é só leitura)

```bash
# o que tens instalado no sistema
which go clang gcc cc make pkg-config
dpkg -l 2>/dev/null | grep -iE 'libgl|libx11|libxcursor|libxrandr|libxinerama|libxi-dev|libxxf86vm|mesa' || echo "(dpkg não disponível ou nada encontrado)"
pkg-config --list-all 2>/dev/null | grep -iE 'gl|x11' || echo "(pkg-config não disponível ou nada encontrado)"

# variáveis de ambiente do Go
go env GOROOT GOPATH GOPROXY GOCACHE CGO_ENABLED

# DISPLAY (para janelas)
echo "DISPLAY=$DISPLAY"
```

Manda-me o output disto junto com o erro do `go run .` — com isto consigo dizer-te exatamente o que falta.

### Tabela de erros prováveis e o que tentar

| Erro contém | Causa provável | O que tentar (por ordem) |
|---|---|---|
| `'GL/gl.h' file not found` | Faltam headers OpenGL | 1. `sudo apt install libgl1-mesa-dev` (se houver root) <br>2. Sem root: pedir ao staff para instalar `xorg-dev libgl1-mesa-dev` (cobre quase tudo do X+GL) <br>3. Verificar com `find / -name "gl.h" 2>/dev/null` se já está noutro sítio |
| `'X11/Xlib.h' file not found` | Faltam headers X11 | 1. `sudo apt install libx11-dev` <br>2. Ou `sudo apt install xorg-dev` (metapacote, instala tudo do X de uma vez) |
| `Xcursor.h`, `Xrandr.h`, `Xinerama.h`, `Xinput2.h`, `Xxf86vm.h` not found | Falta uma lib específica do X | `sudo apt install libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libxxf86vm-dev` (ou `xorg-dev` que apanha tudo) |
| `cgo: C compiler "cc" not found` ou `"clang" not found` | Go não está a encontrar o compilador | 1. `export CC=clang` e tenta de novo <br>2. Ou `export CC=gcc` se for esse o que existe <br>3. Verificar com `which clang gcc cc` |
| `CGO_ENABLED=0` (ou erro do tipo "cgo: not enabled") | CGO desligado | `export CGO_ENABLED=1` e tenta de novo |
| `cannot find module providing package fyne.io/...` | Problema de rede/proxy | 1. `GOPROXY=direct go mod tidy` <br>2. `GOPROXY=https://proxy.golang.org,direct go mod tidy` <br>3. Confirmar que tens internet: `curl -I https://proxy.golang.org` |
| `dial tcp: i/o timeout` no `go mod tidy` | Firewall/proxy da escola | 1. Verificar `echo $http_proxy $https_proxy` <br>2. Se a escola exigir proxy: `export GOPROXY=https://proxy.golang.org,direct` |
| `permission denied` ao escrever em `~/go/...` | Permissões do GOPATH | 1. `go env GOPATH` para ver onde é <br>2. `export GOPATH=$HOME/go-tap && mkdir -p $GOPATH` para usar outra pasta |
| `Unable to open display` ou `cannot connect to X server` ao correr | Sessão sem ambiente gráfico (ex: SSH sem `-X`) | 1. Confirma que estás na sessão física do PC, não por SSH <br>2. Se for SSH: reconecta com `ssh -X` |
| `Failed to initialize EGL` / `GLX` / driver errors | Driver gráfico não suporta | Pouco comum em PCs de escola, mas se acontecer manda o erro |
| `undefined reference to ...` no link | Libs de runtime em falta | Pode acontecer se compilar mas não linkar — copia o erro completo |
| `command not found: go` | Go não está no PATH | 1. `ls /usr/local/go/bin /opt/go/bin 2>/dev/null` para ver se está noutro sítio <br>2. Se sim: `export PATH=$PATH:/usr/local/go/bin` |
| Tudo compila mas a janela não aparece | Erro silencioso de display | Corre com `go run . 2>&1 \| tee saida.log` e manda o `saida.log` |

### Atalho útil

Se tiveres root e quiseres apanhar tudo de uma vez em vez de ir um por um:

```bash
sudo apt install -y golang-go clang xorg-dev libgl1-mesa-dev pkg-config
```

Isto resolve praticamente todos os erros da tabela em simultâneo.

### Se nada do que tentares funcionar

Sem stress — manda-me o erro original + o output do diagnóstico rápido e decidimos juntos se vale a pena insistir no Fyne, mudar para Gio (menos dependências de sistema) ou voltar à GUI web.

---

## Quando acabares

Se tudo correr bem, diz-me — começamos a migração logo.

A pasta `testeFyne/` é descartável, não faz parte do projeto. Apagamos quando estivermos os dois confortáveis com a decisão final.
