class Fermata < Formula
  desc "Debugger for GitHub Actions: pause a failing workflow, fix it, retry one step"
  homepage "https://github.com/aradar46/fermata"
  url "https://github.com/aradar46/fermata/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "aceb213c08aeb5d66b4aee9cfdc082672ff7e26cc9628e2815acebd6b9246260"
  license "MIT"
  head "https://github.com/aradar46/fermata.git", branch: "main"

  depends_on "go" => :build

  # Built from source on the user's machine, deliberately. No macOS binary is
  # published, because none can be exercised end to end before release: macOS
  # cannot run the Linux containers Fermata drives. Compiling here means the
  # binary is native and linked against the user's own toolchain.
  def install
    ldflags = "-s -w -X github.com/aradar46/fermata/cmd.version=#{version}"
    system "go", "build", *std_go_args(ldflags: ldflags)
  end

  def caveats
    <<~EOS
      Fermata runs your workflows in containers and needs a working Docker
      daemon. On macOS that means Docker Desktop, Colima, or OrbStack running
      before you invoke it.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/fermata --version")

    # `run` must fail cleanly on a missing workflow rather than panicking.
    # Docker is not available in the test sandbox, so this is the deepest
    # assertion that can run here.
    output = shell_output("#{bin}/fermata run -W nonexistent.yml 2>&1", 1)
    refute_match "panic:", output
  end
end
