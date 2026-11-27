<script lang="ts">
  // The app's name, written 歌那火 with its furigana, うた・な・び: 歌, song; 那, there for its sound; and 火, fire,
  // read bi as in 花火, hanabi, fireworks. Song fireworks, like the mark. 歌 is in the accent and 火 in the spark,
  // glowing a little. Screen readers hear "Utanabi". size is the kanji's font size.
  let { size = "1.5rem" }: { size?: string } = $props();
</script>

<span class="wordmark" style:font-size={size}>
  <span class="visually-hidden">Utanabi</span>
  <span aria-hidden="true" lang="ja"
    ><ruby class="uta">歌<rt>うた</rt></ruby><ruby class="na">那<rt>な</rt></ruby><ruby class="bi">火<rt>び</rt></ruby
    ></span
  >
</span>

<style>
  .wordmark {
    display: inline-block;
    font-weight: 800;
    line-height: 1.6;
    letter-spacing: 0.02em;
  }

  ruby {
    ruby-position: over;
  }

  rt {
    font-size: 0.36em;
    font-weight: 600;
    letter-spacing: 0.08em;
    color: var(--muted);
  }

  .uta {
    color: var(--link);
  }

  .na {
    color: var(--fg);
  }

  .bi {
    color: var(--spark);
    text-shadow: 0 0 0.35em color-mix(in srgb, var(--spark) 55%, transparent);
  }

  .bi rt {
    text-shadow: none;
  }
</style>
