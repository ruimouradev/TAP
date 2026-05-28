# Teste Gio — passos para o Alexandre

Objetivo: confirmar que o Gio compila e corre num PC da escola. Alternativa ao Fyne (ver `../testeFyne/`), com dependências de sistema diferentes — se um falhar, o outro pode passar.

Corre tudo num PC da escola (Linux), não na tua máquina pessoal.

Podes testar os dois (`testeFyne` e `testeGio`) na mesma sessão, são independentes.

---

## 1. Confirmar que há Go instalado

```bash
go version
```

- Se devolver algo tipo `go version go1.22.x linux/amd64` → segue para o passo 2.
- Se devolver `command not found` → diz-me, sem Go nada disto avança.

## 2. Confirmar que há compilador C

```bash
clang --version
```

(ou `gcc --version` se o `clang` não existir)

O Gio também usa CGO, por isso precisa de um compilador C, tal como o Fyne.

## 3. Ir para a pasta do teste

A partir da raiz do projeto:

```bash
cd testeGio
```

## 4. Descarregar as dependências do Gio

```bash
go mod tidy
```

Vai buscar o Gio e dependências transitivas. Demora 1-2 minutos na primeira vez. Não precisa de root, é tudo no espaço de utilizador (`~/go/pkg/...`).

## 5. Compilar e correr

```bash
go run .
```

### Resultado esperado

Abre uma janela pequena com:
- O texto "If you can read this, Gio works on this machine."
- Um botão "Click me"

Clica no botão → o texto muda. Se isto acontecer, o teste passou.

---

## Se falhar

A maior parte dos erros tem mais que uma solução, podes ir tentando.

### Diagnóstico rápido (corre tudo, é só leitura)

```bash
# o que tens instalado no sistema
which go clang gcc cc make pkg-config
dpkg -l 2>/dev/null | grep -iE 'libgl|libegl|libwayland|libxkb|libx11|vulkan|mesa' || echo "(dpkg não disponível ou nada encontrado)"
pkg-config --list-all 2>/dev/null | grep -iE 'gl|egl|wayland|xkb|vulkan' || echo "(pkg-config não disponível ou nada encontrado)"

# variáveis de ambiente do Go
go env GOROOT GOPATH GOPROXY GOCACHE CGO_ENABLED

# DISPLAY ou Wayland
echo "DISPLAY=$DISPLAY"
echo "WAYLAND_DISPLAY=$WAYLAND_DISPLAY"
```

Manda-me o output disto junto com o erro do `go run .` — com isto consigo dizer-te exatamente o que falta.

### Tabela de erros prováveis e o que tentar

| Erro contém | Causa provável | O que tentar (por ordem) |
|---|---|---|
| `'EGL/egl.h' file not found` | Faltam headers EGL | 1. `sudo apt install libegl1-mesa-dev` (se houver root) <br>2. Sem root: pedir ao staff |
| `'GL/gl.h' file not found` ou `'GLES2/gl2.h' not found` | Faltam headers OpenGL/OpenGL ES | 1. `sudo apt install libgl1-mesa-dev libgles2-mesa-dev` <br>2. Verificar com `find / -name "gl.h" 2>/dev/null` |
| `'wayland-client.h' not found` | Faltam headers Wayland | 1. `sudo apt install libwayland-dev` <br>2. Se a escola não usar Wayland: tentar forçar backend X11 com `export GIOAPI=x11` antes do `go run` |
| `'xkbcommon/xkbcommon.h' not found` | Falta lib do teclado | `sudo apt install libxkbcommon-dev libxkbcommon-x11-dev` |
| `'vulkan/vulkan.h' not found` | Faltam headers Vulkan | `sudo apt install libvulkan-dev` (raro mas pode acontecer em versões recentes do Gio) |
| `cgo: C compiler "cc" not found` ou `"clang" not found` | Go não está a encontrar o compilador | 1. `export CC=clang` e tenta de novo <br>2. Ou `export CC=gcc` se for esse o que existe |
| `CGO_ENABLED=0` (ou erro "cgo: not enabled") | CGO desligado | `export CGO_ENABLED=1` e tenta de novo |
| `cannot find module providing package gioui.org/...` | Problema de rede/proxy | 1. `GOPROXY=direct go mod tidy` <br>2. `GOPROXY=https://proxy.golang.org,direct go mod tidy` |
| `dial tcp: i/o timeout` no `go mod tidy` | Firewall/proxy da escola | 1. Verificar `echo $http_proxy $https_proxy` <br>2. Se a escola exigir proxy: `export GOPROXY=https://proxy.golang.org,direct` |
| `permission denied` ao escrever em `~/go/...` | Permissões do GOPATH | 1. `go env GOPATH` para ver onde é <br>2. `export GOPATH=$HOME/go-tap && mkdir -p $GOPATH` |
| `failed to initialize display` / `connection refused` ao correr | Sessão sem ambiente gráfico | 1. Confirma que estás na sessão física do PC, não por SSH <br>2. Se for SSH: reconecta com `ssh -X` |
| `Failed to create OpenGL context` / `EGL_BAD_DISPLAY` | Driver gráfico sem suporte adequado | Pouco comum em PCs de escola, mas se acontecer manda o erro |
| `command not found: go` | Go não está no PATH | 1. `ls /usr/local/go/bin /opt/go/bin 2>/dev/null` <br>2. Se sim: `export PATH=$PATH:/usr/local/go/bin` |
| Tudo compila mas a janela não aparece | Erro silencioso de display | Corre com `go run . 2>&1 \| tee saida.log` e manda o `saida.log` |

### Atalho útil

Se tiveres root e quiseres apanhar tudo de uma vez:

```bash
sudo apt install -y golang-go clang \
    libgl1-mesa-dev libegl1-mesa-dev libgles2-mesa-dev \
    libwayland-dev libxkbcommon-dev libxkbcommon-x11-dev \
    libvulkan-dev pkg-config
```

Isto resolve praticamente todos os erros da tabela em simultâneo.

### Se nada funcionar

Sem stress — manda-me o erro original + o output do diagnóstico rápido. Se nem Fyne nem Gio passarem nos PCs da escola, voltamos a considerar a GUI web (v1_JS) com a abordagem da bridge.

---

## Comparar com o Fyne

Se correres os dois testes:
- **Ambos passam** → escolhemos Fyne (API mais simples, comunidade maior, melhor para aprender).
- **Só Fyne passa** → Fyne.
- **Só Gio passa** → Gio (mais curva de aprendizagem, mas viável).
- **Nenhum passa** → voltamos à v1_JS (GUI web).

---

## Quando acabares

Tanto se um, ambos ou nenhum passar, partilha o resultado e decidimos juntos.

A pasta `testeGio/` é descartável, não faz parte do projeto. Apagamos quando estivermos confortáveis com a decisão final.
