import dynamic from "next/dynamic";
import InGameScene from "../../../components/home/InGameScene";
import { formatHpPct } from "../model/derive-home-model";
import type { HomeActions, HomeGameView } from "../model/types";

const GuessMap = dynamic(() => import("../../../components/GuessMap"), {
  ssr: false,
});

type HomePageGameProps = {
  game: HomeGameView;
  maxHP: number;
  actions: Pick<
    HomeActions,
    | "placeGuess"
    | "finalizeGuess"
    | "advanceRound"
    | "forfeitMatch"
    | "leaveGame"
  >;
};

export default function HomePageGame({
  game,
  maxHP,
  actions,
}: HomePageGameProps) {
  if (!game.inGame || (game.uiPhase === "match_end" && game.showMatchEndPage)) {
    return null;
  }

  const framedResultMap = game.roundResult ? (
    <div className="h-full w-full overflow-hidden rounded-[22px] border border-white/10 bg-[#162130] shadow-[0_20px_42px_rgba(0,0,0,0.35)]">
      <GuessMap
        mode="result"
        result={game.roundResult}
        interactiveInResult
        resultPlayerAvatars={game.resultPlayerAvatars}
        resultPlayerFallbacks={game.resultPlayerFallbacks}
        resultPlayerBorderColors={game.resultPlayerBorderColors}
      />
    </div>
  ) : null;

  return (
    <InGameScene
      uiPhase={game.uiPhase}
      streetViewSrc={game.streetViewSrc}
      streetViewInteractive={game.streetViewInteractive}
      ruleset={game.ruleset}
      streetNames={game.streetNames}
      showResultStage={game.showResultStage}
      isSingleplayer={game.isSingleplayer}
      isPointsMode={game.isPointsMode}
      partyMode={game.mode === "singleplayer" ? undefined : game.mode}
      backLabel={game.backLabel}
      resultOverlay={
        game.roundResult && game.resultOverlay
          ? {
              ...game.resultOverlay,
              mapNode: framedResultMap,
            }
          : undefined
      }
      roundResults={game.roundResults}
      resultPlayerNames={game.resultPlayerNames}
      resultPlayerAvatars={game.resultPlayerAvatars}
      resultPlayerFallbacks={game.resultPlayerFallbacks}
      participantsById={game.participantsById}
      sides={game.sides}
      hpPct={(hp) => formatHpPct(maxHP, hp)}
      mm={game.mm}
      ss={game.ss}
      isRoundTimerRunning={game.isRoundTimerRunning}
      timerProgressPct={game.timerProgressPct}
      isTimerCritical={game.isTimerCritical}
      isTimerPulseActive={game.isTimerPulseActive}
      resultMode={game.resultMode}
      selfHP={game.selfHP}
      oppHP={game.oppHP}
      totalScore={game.totalScore}
      currentRoundScore={game.currentRoundScore}
      currentRoundDistanceKm={game.currentRoundDistanceKm}
      onForfeit={actions.forfeitMatch}
      onAdvanceRound={actions.advanceRound}
      onLeaveGame={actions.leaveGame}
      canFinalizeGuess={game.canFinalizeGuess}
      canAdvanceRound={game.canAdvanceRound}
      onFinalizeGuess={actions.finalizeGuess}
      guessMapNode={
        <GuessMap
          key={`guess-map-${game.currentRoundId || "none"}`}
          onGuess={actions.placeGuess}
          guess={game.guess}
          mode="guess"
          guessAvatarUrl={game.userAvatar}
          guessAvatarFallback={
            game.participantsById[game.selfUserId]?.avatarFallback || "?"
          }
        />
      }
      resultMapNode={framedResultMap}
      damageMultiplier={game.damageMultiplier}
      guessSubmitted={game.guessSubmitted}
      opponentGuessAlert={game.opponentGuessAlert}
      connectionIssue={game.connectionIssue}
      roundNumber={game.currentRoundNumber}
      totalRounds={game.totalRounds}
      modeName={game.modeName}
      mapName={game.mapName}
      selfUserId={game.selfUserId}
    />
  );
}
