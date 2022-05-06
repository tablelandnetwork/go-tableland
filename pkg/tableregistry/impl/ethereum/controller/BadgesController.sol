// SPDX-License-Identifier: MIT

pragma solidity ^0.8.4;

import "./Controller.sol";

contract BadgesController is TablelandController {

    address private _rigs;
    address private _badges;

    function getPolicy(address caller) public override view returns(TablelandControllerLibrary.Policy memory) {
        string[] memory clauses = new string[](2);

        // Require one of rigs
        clauses[0] = requireOneOfERC721(caller, _rigs, "rig_id");

        // Require one of badges
        clauses[1] = requireOneOfERC721(caller, _badges, "id");

        // Restrict updates to the position column
        string[] memory updatableColumns = new string[](1);
        updatableColumns[0] = "position";

        // Return policy
        return TablelandControllerLibrary.Policy({
            allowInsert: false,
            allowUpdate: true,
            allowDelete: false,
            withCheck: "",
            whereClause: joinClauses(clauses),
            updatableColumns: updatableColumns
        });
    }

    function setRigs(address rigs) public {
        _rigs = rigs;
    }

    function setBadges(address badges) public {
        _badges = badges;
    }
}
