import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { applyLocalization } from '../i18n'
import { PresentationProvider } from '../presentation'
import type { AccountBuffPage, LocalizationCatalog } from '../types'
import { AccountBuffsPage } from './account-buffs-page'
const load=vi.fn()
vi.mock('../../wailsjs/go/main/DesktopApp',()=>({AccountBuffs:(...args:unknown[])=>load(...args)}))
const catalog:LocalizationCatalog={locale:'en',ui:{nav_account_buffs:'Account buffs',account_furniture:'Furniture',account_bond:'Bond bonuses',buff_source_scan_level:'Furniture level {level}',buff_bond_source:'Bond level {level}',buff_no_furniture:'Import furniture levels',buff_no_bond:'No level 10 bond bonus'},stats:{AtkAdd:'Flat ATK',DefAdd:'Flat DEF',CritDamageBase:'CRIT DMG',CritBase:'CRIT'},qualities:{},geometries:{},damage:{},stat_sources:{}}
function fixture():AccountBuffPage {
 const furniture=[{id:'attack',level:2,stats:[{property_id:'AtkAdd',value:4,percent:false}]},{id:'defense',level:3,stats:[{property_id:'DefAdd',value:9,percent:false}]},{id:'critical',level:6,stats:[{property_id:'CritDamageBase',value:.024,percent:true}]},{id:'other-level',level:3,stats:[{property_id:'AtkAdd',value:6,percent:false}]}]
 const character={character_id:1036,name:'Zankou',bond_level:10,bond_bonus_active:true,bond_stats:[{property_id:'CritBase',value:.04,percent:true}]}
 return {detected_furniture_ids:['attack','defense','critical'],furniture,characters:[character,{...character,character_id:1004,name:'Lower bond',bond_level:9,bond_bonus_active:false}]}
}
function mount(){render(<PresentationProvider catalog={catalog}><AccountBuffsPage locale="en"/></PresentationProvider>)}
describe('Account buffs page',()=>{
 beforeEach(()=>{vi.clearAllMocks();applyLocalization('en',catalog);load.mockResolvedValue(fixture())})
 it('shows scanned furniture and level 10 characters without editing controls',async()=>{
  mount()
  expect(await screen.findByText('Furniture level 2')).toBeInTheDocument()
  expect(screen.getByText('Furniture level 3')).toBeInTheDocument()
  expect(screen.getByText('Furniture level 6')).toBeInTheDocument()
  expect(screen.getByText('Zankou')).toBeInTheDocument()
  expect(screen.getByText('Bond level 10')).toBeInTheDocument()
  expect(screen.queryByText('Lower bond')).not.toBeInTheDocument()
  for(const role of ['checkbox','combobox','button','table'])expect(screen.queryByRole(role)).not.toBeInTheDocument()
 })
 it('shows empty states when the latest scan has no eligible bonuses',async()=>{
  const page=fixture();page.detected_furniture_ids=[];page.characters=page.characters.filter(character=>character.bond_level!<10);load.mockResolvedValue(page)
  mount()
  expect(await screen.findByText('Import furniture levels')).toBeInTheDocument()
  expect(screen.getByText('No level 10 bond bonus')).toBeInTheDocument()
  expect(screen.queryByText('Flat ATK')).not.toBeInTheDocument()
 })
 it('uses backend bond activation rather than inferring it from the display level',async()=>{
  const page=fixture();page.characters[1].bond_level=10;load.mockResolvedValue(page)
  mount()
  expect(await screen.findByText('Zankou')).toBeInTheDocument()
  expect(screen.queryByText('Lower bond')).not.toBeInTheDocument()
 })
})
