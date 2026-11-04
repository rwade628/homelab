// LOCAL PATCH over rethink v0.1.4's dist/cloud/devices/WIN_056905_WW.js,
// mounted by ../helmrelease.yaml. Changes, all from upstream PRs by the same
// author, hand-ported to the compiled JS:
//   - anszom/rethink#283: setProperty('power') -> setProperty('climate-power')
//   - anszom/rethink#284 (mode/preset parts only): dry is code 1 (v0.1.4 sent
//     8, which is Energy Saver), fan 'medium' (4), Energy Saver as preset
//     'eco', and no swing (0x322) in mode writes. Our LW8022IVSM advertises
//     0x2c1=263 (cool/dry/fan/energy saver, no heat), 0x2c2=84 (low/medium/
//     high) and never reports 0x322. #284's caps-driven config is NOT ported:
//     HA rejects v0.1.4's discovery anyway, so HA's mqtt.yaml defines the
//     entity and must match this file's topics.
// Delete this file, its configMapGenerator entry and its mount once a rethink
// release past v0.1.4 includes #283 and #284.

import TLVDevice from './tlv_device.js';
import { allowExtendedType } from '../../util/casting.js';
import HADevice from './base.js';
import { Enum } from '../../util/enum.js';
// 0x1f9 codes
const MODES = Enum.of({
    cool: 0,
    dry: 1,
    fan_only: 2,
    heat: 4,
});
// Energy saver. HA has no such HVAC mode, so it is published as cool with preset 'eco'.
const MODE_ENERGY_SAVER = 8;
const FAN_MODES = Enum.of({
    low: 2,
    medium: 4,
    high: 6,
});
/**
 * LG Air Conditioner Model LW1823HRSM
 */
export default class Device extends TLVDevice {
    constructor(HA, thinq, meta) {
        super(HA, thinq);
        const config = allowExtendedType({
            ...HADevice.config(meta),
            name: 'LG Air Conditioner',
            components: {
                climate: {
                    platform: 'climate',
                    unique_id: '$deviceid-climate',
                    name: null,
                    temperature_unit: 'C',
                    temp_step: 0.5,
                    precision: 0.5,
                    modes: ['off', 'cool', 'dry', 'fan_only'],
                    fan_modes: FAN_MODES.options,
                    preset_modes: ['eco'],
                    preset_mode_state_topic: '$this/climate-preset_mode',
                },
            },
        });
        this.addField(config, {
            id: 0x1fd,
            name: 'current_temperature',
            comp: 'climate',
            state_topic: 'topic',
            writable: false,
            read_xform: (raw) => raw / 2,
        });
        this.addField(config, {
            id: 0x1fe,
            name: 'temperature',
            comp: 'climate',
            read_xform: (raw) => raw / 2,
            write_xform: (valStr) => {
                const val = Number(valStr);
                // set val to min: 61F, max: 86F
                const minCel = 16;
                const maxCel = 30.0;
                if (val < minCel)
                    return minCel * 2;
                if (val > maxCel)
                    return maxCel * 2;
                return Math.round(val * 2);
            },
            write_attach: [0x1f9, 0x1fa],
        });
        this.addField(config, {
            id: 0x1f7,
            name: 'power',
            comp: 'climate',
            readable: false,
            write_xform: (val) => (val === 'ON' ? 1 : 0),
            write_attach: (raw) => (raw ? [0x1f9] : []),
            read_xform: (raw) => (raw ? 'ON' : 'OFF'),
            read_callback: (val) => {
                // update 'mode' instead
                this.processKeyValue(0x1f9, this.raw_clip_state[0x1f9]);
                return false;
            },
        });
        this.addField(config, {
            id: 0x1f9,
            name: 'mode',
            comp: 'climate',
            read_xform: (raw) => {
                this.publishProperty('climate-preset_mode', raw === MODE_ENERGY_SAVER ? 'eco' : 'none');
                if (this.raw_clip_state[0x1f7] === 0)
                    return 'off';
                if (raw === MODE_ENERGY_SAVER)
                    return 'cool';
                return MODES.map(raw);
            },
            // 'eco' is not an HA mode; the preset field below writes it through here
            write_xform: (val) => {
                this.setProperty('climate-power', val === 'off' ? 'OFF' : 'ON');
                if (val === 'eco')
                    return MODE_ENERGY_SAVER;
                return MODES.unmap(val);
            },
            write_attach: [0x1f7, 0x1fa, 0x1fe],
        });
        this.addField(config, {
            name: 'preset_mode',
            comp: 'climate',
            readable: false,
            write_xform: (val) => (val === 'eco' ? MODE_ENERGY_SAVER : 0),
            write_callback: (raw) => {
                const current = this.raw_clip_state[0x1f9];
                if (raw === MODE_ENERGY_SAVER)
                    this.setProperty('climate-mode', 'eco');
                // leaving eco: back to plain cool; 'none' in any other mode changes nothing
                else if (current === MODE_ENERGY_SAVER)
                    this.setProperty('climate-mode', 'cool');
                return false;
            },
        });
        this.addField(config, {
            id: 0x1fa,
            name: 'fan_mode',
            comp: 'climate',
            read_xform: (raw) => FAN_MODES.map(raw),
            write_xform: (val) => FAN_MODES.unmap(val),
            write_attach: [0x1f9, 0x1fe],
        });
        this.setConfig(config);
    }
    isCapsResponse(tlvArray) {
        /* eeprom checksum */
        return tlvArray.some(({ t, v }) => t === 0x2da);
    }
    isValuesResponse(tlvArray) {
        /* power */
        return tlvArray.length >= 10 && tlvArray.some(({ t, v }) => t === 0x1f7);
    }
}
