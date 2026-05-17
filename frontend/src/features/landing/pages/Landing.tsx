import { AutoHealSection } from '@/features/landing/components/AutoHealSection'
import { CapabilitiesSection } from '@/features/landing/components/CapabilitiesSection'
import { EngineeringSection } from '@/features/landing/components/EngineeringSection'
import { Footer } from '@/features/landing/components/Footer'
import { HeroSection } from '@/features/landing/components/HeroSection'
import { LandingHeader } from '@/features/landing/components/LandingHeader'
import { ReplacementSection } from '@/features/landing/components/ReplacementSection'
import { ScenarioSection } from '@/features/landing/components/ScenarioSection'
import { ScreenshotSection } from '@/features/landing/components/ScreenshotSection'
import { WorkflowSection } from '@/features/landing/components/WorkflowSection'

export default function Landing() {
  return (
    <div className="min-h-screen bg-slate-50 text-slate-950">
      <LandingHeader />
      <main>
        <HeroSection />
        <CapabilitiesSection />
        <ScreenshotSection />
        <AutoHealSection />
        <EngineeringSection />
        <WorkflowSection />
        <ReplacementSection />
        <ScenarioSection />
      </main>
      <Footer />
    </div>
  )
}
