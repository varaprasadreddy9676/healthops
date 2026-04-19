import { Composition } from "remotion";
import { HealthOpsDemo } from "./HealthOpsDemo";

const FPS = 30;
// Intro: 4s, Recording: ~55s, Outro: 4s = ~63s total
const DURATION_SECONDS = 63;

export const RemotionRoot: React.FC = () => {
  return (
    <Composition
      id="HealthOpsDemo"
      component={HealthOpsDemo}
      durationInFrames={DURATION_SECONDS * FPS}
      fps={FPS}
      width={1440}
      height={900}
    />
  );
};
