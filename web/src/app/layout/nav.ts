import { Compass, FileAudio, LayoutDashboard, Library, Search, Upload, type LucideIcon } from 'lucide-react';

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  end?: boolean;
}

export const primaryNav: NavItem[] = [
  { to: '/', label: 'Discover', icon: Compass, end: true },
  { to: '/search', label: 'Search', icon: Search },
  { to: '/samples', label: 'Samples', icon: FileAudio },
  { to: '/library', label: 'Library', icon: Library },
];

export const publisherNav: NavItem[] = [
  { to: '/publisher', label: 'Dashboard', icon: LayoutDashboard, end: true },
  { to: '/publisher/upload', label: 'Upload release', icon: Upload },
];
