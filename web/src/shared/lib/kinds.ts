import {
  AudioWaveform,
  Boxes,
  Brain,
  FileAudio,
  Gauge,
  LayoutTemplate,
  Music2,
  Piano,
  SlidersHorizontal,
  Sparkles,
  type LucideIcon,
} from 'lucide-react';
import type { ModuleClass, PackageKind } from '@/shared/types/domain';

export const KIND_META: Record<PackageKind, { label: string; plural: string; icon: LucideIcon }> = {
  'sample-pack': { label: 'Sample pack', plural: 'Samples', icon: FileAudio },
  'preset-pack': { label: 'Preset pack', plural: 'Presets', icon: SlidersHorizontal },
  instrument: { label: 'Instrument', plural: 'Instruments', icon: Piano },
  effect: { label: 'Effect', plural: 'Effects', icon: Sparkles },
  'midi-tool': { label: 'MIDI tool', plural: 'MIDI Tools', icon: Music2 },
  utility: { label: 'Utility', plural: 'Utilities', icon: Gauge },
  'project-template': { label: 'Project template', plural: 'Templates', icon: LayoutTemplate },
  model: { label: 'Model', plural: 'Models', icon: Brain },
};

export const MODULE_CLASS_META: Record<ModuleClass, { label: string; io: string; icon: LucideIcon }> = {
  instrument: { label: 'Instrument', io: 'MIDI → Audio', icon: Piano },
  'audio-effect': { label: 'Audio effect', io: 'Audio → Audio', icon: Sparkles },
  'note-effect': { label: 'Note effect', io: 'MIDI → MIDI', icon: Music2 },
  'note-generator': { label: 'Note generator', io: 'Transport → MIDI', icon: Music2 },
  'audio-generator': { label: 'Audio generator', io: 'Transport → Audio', icon: AudioWaveform },
  analyzer: { label: 'Analyzer', io: 'Audio → Analysis', icon: Gauge },
  utility: { label: 'Utility', io: 'Audio/MIDI → Audio/MIDI', icon: SlidersHorizontal },
  converter: { label: 'Converter', io: 'Audio ↔ MIDI', icon: Brain },
  macro: { label: 'Macro', io: 'Graph', icon: Boxes },
  visualizer: { label: 'Visualizer', io: 'Audio/MIDI → UI', icon: AudioWaveform },
};
