# The environment the screenshot goldens are recorded in — and the only one they are compared in.
#
# Pictures are not portable across machines the way a JSON body is. Recorded on macOS and verified
# on Linux, the same five screens differed in 2.5-8.6% of their pixels with channel deviations up to
# 214 out of 255: not antialiasing, but different glyphs from a different font stack. A tolerance
# wide enough to absorb that is wide enough to absorb any change worth catching, which is how a
# regression test quietly stops regressing.
#
# So the environment is the golden, not the machine. Both halves of the check — recording and
# verifying, on a laptop and in CI — run in this image.
#
# The base is pinned by digest because a moving tag moves the fonts with it. Package versions are
# not pinned: apt drops superseded versions from the archive, so pinning them buys a build that
# breaks on a schedule instead of a check that goes red and says what changed.
FROM eclipse-temurin@sha256:2ed7aff176420e609546a76378a7dc0f33ddff5b0f9b842d6acc89ce9126d337

# fontconfig and DejaVu are what the text is drawn with; libGL and libX11 are what Skia refuses to
# load without, with an error that names neither. Both were found by ldd on the native library
# rather than guessed from a list of usual suspects.
RUN apt-get update -qq \
 && apt-get install -y -qq --no-install-recommends \
      fontconfig fonts-dejavu-core libfreetype6 libgl1 libx11-6 \
 && rm -rf /var/lib/apt/lists/*

ENV GRADLE_USER_HOME=/gradle
WORKDIR /w/client
