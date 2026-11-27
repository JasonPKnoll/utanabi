# Analyzer workarounds

The app reads lyrics with two tools that weren't made for them. kagome splits a line into tokens using the IPA dictionary. JMdict gives the meanings. Lyrics break both of them in predictable ways: English mixed into a line, numbers in digits, katakana used for effect, spoken contractions, set phrases. This page lists every place where the app corrects for that, and every case it still gets wrong.

Rules for adding to it:

- **A workaround is a general rule, never a fix for one song or one word.** It names the pattern it corrects (a noun, then が, を, or に, then a verb), and the example only shows the pattern.
- **Every workaround has a row here and a test case** with its example, so a later change can't quietly undo it. A rule that has an exception (学校でも stays "at" + "also") tests the exception too.
- **When a fix isn't possible yet, add it to "Not fixed" instead,** with the reason.

The rules themselves are in `internal/features/songs`, and the full description of words, particles, and meanings is in [the data model](data-model.md#finding-words).

## Workarounds

### Grouping tokens into words

| Problem | Example | What the app does | Code | Test |
| --- | --- | --- | --- | --- |
| kagome cuts English into pieces, and each piece looked like a word whose English was itself. | It doesn't matter → It, doesn, ', t | Text with no kana or kanji belongs to no word. An English phrase stays whole, and an all-English line reads as an ordinary line. | `words.go` `hasJapanese` | `TestFindWords` ("It doesn't matter if it's Sunday", "Monday 星が光る") |
| Digits have no reading and no JMdict entry, which spells numbers in kanji. | 1番 read as 1 + ばん, "1ban" | A number in digits and its counter are one word, looked up in kanji (1 to 10). The reading comes from JMdict and is split at the counter, so 1 gets いち. When it doesn't split, all of it goes over the number. | `words.go` `numberWord`, `meanings.go` `readNumber` | `TestFindWords` (1番, 12番), `TestNumbersInDigitsAreReadLikeTheirKanji` (1番 いち + ばん, 2人 ふたり) |
| A question word and でも mean "any-", but kagome splits them, sometimes into で and も. | なん + でも, だれ + で + も | A question word with でも is one word, looked up whole: なんでも "anything". | `words.go` `demoAfter` | `TestFindWords` (だれでも, 針でもなんでも) |
| In XでもYでも ("X or Y, whatever"), kagome reads the first でも as で "at" and も "also". | 針でもなんでも | で and も are one particle, でも, when a noun or pronoun with でも follows straight after. Otherwise they stay apart. | `words.go` `takeWord` | `TestFindWords` (針でもなんでも; 学校でも stays apart) |
| このまま is one dictionary word, but kagome splits it. | この + まま | この, その, あの, or どの with まま is one word. | `words.go` `takeWord` | `TestFindWords` (このまま) |
| kagome only joins a noun to する when it marks the noun as a する noun. | おサラバ + する | Any ordinary noun with する straight after it is a する verb, with its prefix kept. | `words.go` `attaches` | `TestFindWords` (おサラバする) |
| よか is spoken より ("than"), but kagome reads よ and か, emphasis and a question. | するよか | よ with か straight after is one particle, よか, read as より. | `words.go` `takeWord`, `particles.go` | `TestFindWords` (するよか) |
| kagome splits set phrases that JMdict has whole, so each part takes its own, wrong meaning. | 気 "spirit" + が + つく "be lit" | A noun, then が, を, or に, then a verb, is one word when JMdict has the phrase: 気がつく "notice". It conjugates as its verb. | `meanings.go` `joinPhrases` | `TestWordsTakeTheirMeaningFromTheDictionary` (気がつく, 気が付いた) |
| Spoken Japanese drops the particle of a set phrase. | 迷惑 "trouble" + かける "hang up" | A noun straight before a verb is one word when JMdict has the phrase with を or が put back: 迷惑かける is 迷惑をかける "cause trouble". | `meanings.go` `joinPhrases` | `TestWordsTakeTheirMeaningFromTheDictionary` (迷惑かけた) |
| A question word and も mean every- or no-, but kagome splits them. | なん + も "what" + "also" | A question word with も straight after is one word, looked up whole: なんも "(not) anything", だれも "no one". | `words.go` `takeWord` | `TestFindWords` (なんもねえ, だれも知らない) |
| The spoken ねえ for ない reads as the particle ね, an interjection, or the auxiliary that only follows a verb. | なんもねえ "(right?)", 金がねえ | ねえ after も or が, and ない after any particle but て, is the adjective ない "there isn't". After は, ねえ is left alone: it is as often ね holding the listener (僕はねえ、). | `words.go` `spokenNai` | `TestFindWords` (なんもねえ, 金がねえ; 僕はねえ stays) |
| An adjective ending in ない, said with ねえ, comes apart. | はんぱねえ as はん "seal" + ぱねえ; 仕方ねえ | Two words, or one, ending in ねえ (with よ, ね, or the like sung onto it) are one adjective when JMdict has it with ない: 半端ない, 仕方ない. Verbs are left out, so 知らねえ stays 知る. JMdict has some whole, so なんもねえ is なんもない "there is nothing". | `spoken.go` `joinSpokenNai` | `TestWordsTakeTheirMeaningFromTheDictionary` (はんぱねえよ), `TestSpokenFormsBecomeTheWordsTheyStandFor` (仕方ねえ) |
| A verb made from a katakana word comes apart, and its ending reads as other words. | オナってる as オナ + って "(quote)" + る | A katakana noun followed by a verb ending (る, った, って, ってる, ってた, っている, らない, らねえ) is one godan verb when JMdict has the word with る. Its tokens are marked as a verb and its pieces, so the romaji writes it as one word. テヘペロって stays a quote, since JMdict has no テヘペロる. | `spoken.go` `joinSlangVerbs` | `TestSpokenFormsBecomeTheWordsTheyStandFor` (ググってる, ググった, テヘペロって) |
| An adjective said roughly, its あい or おい turned into ええ, reads as a command. | 臭え as 臭う "be fragrant!" | A godan う verb in its え command form is the adjective when JMdict has its kanji with い as one whose reading turns into the spoken form: 臭え is 臭い, read くせえ. The reading is corrected for furigana and romaji. | `spoken.go` `spokenAdjective` | `TestSpokenFormsBecomeTheWordsTheyStandFor` (指が臭え) |
| ん between two nouns is の said quickly, and read as a negative. | 胸んなか | ん between two nouns, or marked as a case particle, is の: `no-possession`, "'s". | `words.go` `takeWord`, `particles.go` | `TestFindWords` (胸んなか) |

### Particle uses

| Problem | Example | What the app does | Code | Test |
| --- | --- | --- | --- | --- |
| の stands in for が inside a describing phrase, and read as "'s". | 変わることのない愛 | の before a verb or ない marks the subject: `no-subject`, "(subject)". | `particles.go` | `TestFindWords` (変わることのない愛) |
| よ after a noun can call out to it, and read as emphasis. | 鏡よ鏡よ | よ after a noun is `yo-calling` when the line goes on past it with a word that isn't a particle. At the end of a line it stays emphasis. | `particles.go` `callsOut` | `TestFindWords` (鏡よ鏡よ この世; 大丈夫だよ stays emphasis) |
| に before する read as a place. | 孤独にして "in" | に before する, like before なる, is `ni-change`, "into": make it lonely. | `particles.go` `niUse` | `TestFindWords` (孤独にして) |

### Choosing a meaning

| Problem | Example | What the app does | Code | Test |
| --- | --- | --- | --- | --- |
| The common-words JMdict left out many words lyrics use. | 浮つく, 纏う, この世 | `dictionary.db` is built from all of JMdict. | `dictbuild/source.go` | the dictionary build |
| Lyrics write words in katakana for effect, which JMdict lists in hiragana. | ホラ, おサラバ | A form written only in kana also matches the same word in the other kana. | `dictionary/sqlite.go` `matchingEntries` | `TestLookupFiltersByReading` (サガス, さガす, ソラ) |
| A pronoun matched a noun that sounds the same. | なん as 難 "difficulty" | A pronoun prefers a pronoun sense (`pn`), then a noun sense. | `meanings.go` `senseFit` | `TestPickPrefersAFittingSenseThenTheClassThenKana`, `TestWordsTakeTheirMeaningFromTheDictionary` (なんだ) |
| A common word lost to one that's only spelled the same way. | ホラ as 洞 "hollow" | An entry with the spelling exactly as the line writes it wins (法螺 is spelled ホラ). | `meanings.go` `pick` | `TestPickPrefersAFittingSenseThenTheClassThenKana` |
| A rare word marked "usually kana" beat a common word. | だれ as 垂れ "sauce" | An entry whose spelling of the word is in common use beats "usually kana". 誰's だれ is common; 垂れ's common spelling is たれ. | `meanings.go` `pick` | `TestPickPrefersAFittingSenseThenTheClassThenKana` |
| A noun used as an adverb matched a noun that sounds the same. | いつかきっと, いつか as 五日 "5th day of the month" | A noun kagome says can be an adverb, with no particle or copula after it, prefers an entry with an adverb sense: いつか "sometime". With a particle it stays a noun, so 全てが is "everything" and 全て忘れて "entirely". | `meanings.go` `usedAsAdverb`, `senseFit` | `TestWordsTakeTheirMeaningFromTheDictionary` (いつかきっと, 全てが, 全て忘れて) |
| A katakana number in a count or chant read as another word. | サン as "Sun" in サン！ヨン！ゼロワン！ | A katakana word next to a number, with only punctuation between, takes a number entry when it has one. It repeats until nothing changes, so サン！サン！ゼロ turns both. サンが昇る stays "Sun". | `meanings.go` `countingRuns` | `TestWordsTakeTheirMeaningFromTheDictionary` (サン！ヨン！ゼロワン！, サン！サン！ゼロ, サンが昇る) |
| A godan command read as a potential verb's stem or command. | がんばれ as 頑張れる "can persevere" | A potential verb whose only step is its bare stem or its command is the godan verb's command, spelled the same: がんばれ, がんばれよ. | `meanings.go` `potential` | `TestPotentialVerbsBecomeTheirGodanVerb` (がんばれ がんばれ, がんばれよ 俺ら) |

### The English under each word

| Problem | Example | What the app does | Code | Test |
| --- | --- | --- | --- | --- |
| A word without a meaning showed itself again. | げんまん under げんまん | The English is left empty, unless the dictionary form adds something (探す under 探して). | `explain.go` `wordGloss` | `TestWordGlossLeavesOutAWordThatOnlyRepeatsItself` |
| A negative word showed its plain meaning. | 要らない "be needed" | A content word with the `negative` step says "not": "not be needed". | `explain.go` `wordGloss` | `TestLineGlossIsWordByWord` (消えない) |
| ない alone showed "nonexistent", or itself as a grammar word. | なんもねえ "anything · ない" | The adjective ない shows "not there", and the auxiliary "not". | `explain.go` `contentGlosses`, `auxiliaryGlosses` | `TestLineGlossIsWordByWord` (なんもねえ) |
| "to" was cut from every meaning, not only verbs'. | 半端ない "a great extent" | Only a verb loses its "to": "shine", but "to a great extent". | `explain.go` `glossOf` | `TestLineGlossIsWordByWord` (はんぱねえ) |
| Some words mean something else before a negative. | もう要らない, もう "already" | もう is "anymore" and まだ is "yet" when a negative comes later in the line. | `explain.go` `beforeNegative` | `TestLineGlossIsWordByWord` (もう要らない; もう行く unchanged) |
| One grammar point reads differently by what's around it. | 死ぬの, ダサいの, いいのよ, 気がつくなんて | Nominalizer の is "(-ing)" after a verb and "(one)" otherwise; の before よ or ね is "(it's that)"; なんて after a clause is "(to think…)". | `explain.go` `contextGloss` | `TestLineGlossIsWordByWord` |

## Not fixed

| Problem | Example | Why not yet |
| --- | --- | --- |
| Gikun (義訓): a word the lyrics give a reading of their own shows its dictionary reading. | 羅針盤 written with コンパス over it shows らしんばん | The lyricist's furigana is part of the official lyrics, but plain-text lyrics such as LRCLIB's drop it, and nothing else in the text shows it. You add it back in the word sheet ("Sung differently from how it's written?"); see `sung_readings` in [data model](data-model.md). Lyrics that write the reading in brackets, 羅針盤(コンパス), could be read automatically, but none saved so far do. |
| Spoken contractions kagome doesn't know come apart wrongly. | もうしたない (したくない, "don't want to anymore") reads as 申す "say" | kagome makes もうし one token, so there's nothing to join. It needs a list of contractions to expand before analysis. |
| ホラ the interjection ("look!") reads as 法螺 "boasting". | ホラ見て | kagome tags an unknown katakana word as a noun, so the interjection ほら never fits. |
| A word's meaning is the first sense that fits its part of speech, not the one the line means. | 声 in a line about an opinion is still "voice" | Choosing by meaning needs more than the tokens around the word. |
| られる is always `passive`, and `causative-passive` is never chosen. | 食べさせられる reads as causative, then passive | Telling passive, potential, and respectful apart needs the sentence's meaning. |
| Particle uses read only the neighbouring tokens. | `wa-contrast`, `wo-through`, `ni-agent`, `de-means` are never guessed; に after a time word kagome doesn't mark is `ni-place` | Same as above. 空の上に is `ni-place` only because 上 is on the list of place words. |
| The potential-verb check trusts the dictionary. | An ichidan verb missing from JMdict whose godan twin is in it would be read as a potential | JMdict is the only list of verbs the app has. |
| A joke spelling reads as the word it looks like. | すめん (すまん, "sorry") as 済む "not finish" | Nothing tells it from a real form of the verb. |
| で after a person is always `de-place`. | 今日もあなたで, "with you", reads "at" | `de-means` needs the sentence's meaning. |
| A word kept whole reads as a phrase, a word in pieces as words. | なんもねえ is "there is nothing", but 何もない is "anything · not there" | kagome reads ない as an adjective, which the ねえ join leaves out so verbs' negatives stay theirs. |
| Other kagome mistakes carry through. | In 2人で it reads で as the copula | They're kagome's own; each needs its own rule once it shows up in a song. |
| A song saved earlier keeps its words when these rules improve. | Shinunoga E-Wa saved before the fixes | Saved words are never re-analysed; re-adding the song analyses it again. Words that had no meaning do fill in when the song is opened. |
| A line whose tokens no longer match a new analysis keeps no words, and is tried again each time the song opens. | A line of only punctuation | There are no words to save for it. |
