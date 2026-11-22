import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import en from '../../../data/presentation/en.json'
import { applyLocalization } from '../i18n'
import { PresentationProvider } from '../presentation'
import type { LocalizationCatalog, Result } from '../types'
import { BuildBuffs } from './build-buffs'

const labels:LocalizationCatalog={...en,locale:'en',stats:{AtkAdd:'ATK',AtkUp:'ATK %',CritDamageBase:'CRIT DMG',HPMaxUp:'HP'},damage:{},abilities:{ultimate:'Ultimate'},stat_sources:{weapon_conditional:'Arc passive'}}
function result():Result {
  return {stats:{sources:{}},team_allies:[{character_id:1003},{character_id:1042}],buff_effects:[{kind:'skill',id:'ultimate',character_id:1003,detail:'sakiri_skill',trigger:'ultimate',included:true,source_names:['Sakiri'],bonus:{property_id:'AtkAdd',value:300,percent:false}}],ally_buffs:[
    {id:'extracted_attack',character_id:1003,character_name:'Sakiri',ability_name:'Ultimate',property_id:'AtkAdd',percent:false,value:300,duration_seconds:20,condition:'After casting the Ultimate',included:true,legacy_effect_id:'ultimate'},
    {id:'extracted_critical',character_id:1042,character_name:'Blackbird',property_id:'CritDamageBase',percent:true,value:.2,condition:'While this character is on the team',included:false},
  ]} as Result
}
function mount(value:Result){render(<PresentationProvider catalog={labels}><BuildBuffs result={value}/></PresentationProvider>)}
describe('compact teammate buffs',()=>{
  beforeEach(()=>applyLocalization('en',labels))
  it('shows source, flat/percent amount, condition and projection status without duplicate skill rows',()=>{
    mount(result())
    expect(screen.getByText('Teammate buffs')).toBeInTheDocument()
    expect(screen.getAllByText('Sakiri')).toHaveLength(1)
    expect(screen.getByText('+300 ATK')).toBeInTheDocument()
    expect(screen.getByText('+20% CRIT DMG')).toBeInTheDocument()
    expect(screen.getByText(/Ultimate · After casting the Ultimate/)).toBeInTheDocument()
    expect(screen.getByText('In buffed preview')).toBeInTheDocument()
    expect(screen.getByText('Not included')).toBeInTheDocument()
    expect(screen.queryByText('extracted_attack')).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
    expect(screen.queryByText('No temporary effects included.')).not.toBeInTheDocument()
  })
  it('keeps unknown values unavailable and preserves a valid zero',()=>{
    const value=result();delete value.ally_buffs![0].value
    value.ally_buffs![1].value=0
    mount(value)
    expect(screen.getByText(/Value unavailable/)).toBeInTheDocument()
    expect(screen.getByText('+0% CRIT DMG')).toBeInTheDocument()
  })
  it('prints the percentage unit once',()=>{
    const value=result();value.ally_buffs![1].property_id='AtkUp'
    mount(value)
    expect(screen.getByText('+20% ATK')).toBeInTheDocument()
  })
  it('preserves the precision of fractional percent bonuses',()=>{
    const value=result();value.ally_buffs![1].value=.0375;value.ally_buffs![1].per_stack=true
    mount(value)
    expect(screen.getByText('+3.75% CRIT DMG · per stack')).toBeInTheDocument()
  })
  it('preserves the previous presentation for saved results without summaries',()=>{
    const value=result();delete value.ally_buffs
    mount(value)
    expect(screen.queryByText('Teammate buffs')).not.toBeInTheDocument()
    expect(screen.getByText('Sakiri · Ultimate')).toBeInTheDocument()
  })
  it('keeps personal and equipment bonuses alongside teammate summaries',()=>{
    const value=result();value.stats.sources.weapon_conditional={AtkAdd:25};value.weapon={name:'Selected Arc'} as Result['weapon']
    mount(value)
    expect(screen.getByText('Arc passive')).toBeInTheDocument()
    expect(screen.getByText('Selected Arc')).toBeInTheDocument()
    expect(screen.getByText('+25 ATK')).toBeInTheDocument()
    expect(screen.getByText('Teammate buffs')).toBeInTheDocument()
  })
  it('labels effects with alternative damage tags as limited to matching hits', () => {
    const value = result()
    value.buff_effects = [{
      kind: 'arc', id: 'filtered', character_id: 1057, detail: '', trigger: 'redirect_skill', included: true,
      damage_tags_any: ['State.Damage.Melee', 'State.Damage.Branch'],
    }]
    mount(value)
    expect(screen.getByText(en.ui.arc_condition_matching_damage)).toBeInTheDocument()
  })
})
